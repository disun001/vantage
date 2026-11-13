// Package chatui is the Bubble Tea view behind `vantage chat`: a transcript
// of stored events above a multi-line input. It drives a chat through the
// small Driver interface and never touches tmux or the database itself.
package chatui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/disun001/vantage/internal/store"
)

// Driver is the part of a chat the view needs.
type Driver interface {
	Send(ctx context.Context, text string) error
	Interrupt(ctx context.Context) error
	Close() error
}

// Config describes one chat view.
type Config struct {
	// Open creates the Driver. onEvent must be passed on to the chat so each
	// stored event reaches the view. history reads what is already stored.
	Open    func(onEvent func(store.Event)) (Driver, error)
	History func(ctx context.Context) ([]store.Event, error)
	// Title is shown in the header (for example the chat id and cwd).
	Title string
}

// Run shows the chat until the user quits, then closes the driver.
func Run(ctx context.Context, in *os.File, out io.Writer, cfg Config) error {
	var prog atomic.Pointer[tea.Program]
	var live atomic.Bool
	// Events recorded while the chat opens (an interrupted turn) are already in
	// History, so they are dropped until the program runs.
	drv, err := cfg.Open(func(e store.Event) {
		if p := prog.Load(); p != nil && live.Load() {
			p.Send(eventMsg(e))
		}
	})
	if err != nil {
		return err
	}
	hist, err := cfg.History(ctx)
	if err != nil {
		return errors.Join(err, drv.Close())
	}
	m := newModel(ctx, drv, cfg.Title, hist, lipgloss.HasDarkBackground())
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen())
	prog.Store(p)
	live.Store(true)
	_, runErr := p.Run()
	if runErr != nil {
		runErr = fmt.Errorf("chat view: %w", runErr)
	}
	return errors.Join(runErr, drv.Close())
}

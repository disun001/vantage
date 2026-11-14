// Package tui is the full-screen interface: a session sidebar on the left
// and an embedded tmux client on the right.
//
// tmux stays the source of truth. This package only lists sessions, picks
// one, and forwards keys; it never keeps its own copy of window state.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/tmux"
)

// Run starts the TUI and blocks until the user quits.
func Run(cfg Config) error {
	cfg.Client.ApplyPaneBorders(tmux.PaneBorderColors{Inactive: borderIdle, Active: borderActive, Label: mocSub})
	h := &hub{}
	p := tea.NewProgram(newModel(cfg, h), tea.WithAltScreen(), tea.WithMouseCellMotion())
	h.send = p.Send
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

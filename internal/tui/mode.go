package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/tmux"
)

// modeEvery is how often the embedded client's mode is read while the
// terminal has focus. PREFIX only lasts as long as a key chord, so it is fast.
const modeEvery = 200 * time.Millisecond

type (
	modeTickMsg struct{}
	modeMsg     struct {
		pid  int // client the mode belongs to
		mode tmux.Mode
	}
)

func modeTick() tea.Cmd {
	return tea.Tick(modeEvery, func(time.Time) tea.Msg { return modeTickMsg{} })
}

// modeCmd reads the mode of the embedded client. Only the terminal can be in a
// tmux mode, so it asks nothing while the sidebar has focus.
func (m model) modeCmd() tea.Cmd {
	if m.term == nil || m.tty == "" || m.focus != focusTerm {
		return nil
	}
	c, tty, pid := m.cfg.Client, m.tty, m.term.PID()
	return func() tea.Msg {
		mode, err := c.Mode(tty)
		if err != nil {
			return modeMsg{pid: pid}
		}
		return modeMsg{pid: pid, mode: mode}
	}
}

// modeTag is the label and color of the title pill, like tmux's PREFIX, COPY,
// SYNC and TMUX tags. The sidebar having focus wins: vantage, not tmux, is
// taking the keys.
func (m model) modeTag() (label, bg string) {
	switch {
	case m.focus == focusSidebar:
		return "SIDEBAR", mocLavender
	case m.mode.Prefix:
		return "PREFIX", mocYellow
	case m.mode.Copy:
		return "COPY", mocBlue
	case m.mode.Sync:
		return "SYNC", mocRed
	}
	return "VANTAGE", mocGreen
}

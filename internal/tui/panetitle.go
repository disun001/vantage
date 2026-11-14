package tui

import (
	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/workspace"
)

// paneTitleMsg is the active pane of the embedded client and the custom name
// it has, read so the rename prompt can start with it.
type paneTitleMsg struct {
	pane, title string
	err         error
}

// paneTitleCmd looks up the pane alt+r renames: the active pane of the
// embedded client, whichever side of the screen has focus.
func (m model) paneTitleCmd() tea.Cmd {
	c, tty, pid := m.cfg.Client, m.tty, m.term.PID()
	return func() tea.Msg {
		tty, err := clientTTY(c, tty, pid)
		if err != nil {
			return paneTitleMsg{err: err}
		}
		pane, _, err := c.ActivePane(tty)
		if err != nil {
			return paneTitleMsg{err: err}
		}
		title, err := c.PaneTitle(pane)
		return paneTitleMsg{pane: pane, title: title, err: err}
	}
}

// onPaneTitle opens the rename prompt, or says why it cannot.
func (m model) onPaneTitle(msg paneTitleMsg) model {
	if msg.err != nil {
		m.notice = ansi.Strip(msg.err.Error())
		return m
	}
	m.notice = ""
	m.prompt = &prompt{kind: promptRenamePane, target: msg.pane, text: msg.title, old: msg.title}
	return m
}

// renamePaneCmd names the pane in the background. An empty name clears it; a
// name that did not change is left alone.
func (m model) renamePaneCmd(p prompt) tea.Cmd {
	c, title := m.cfg.Client, p.text
	if title == p.old {
		return nil
	}
	return func() tea.Msg {
		msg := manageMsg{op: p.kind, from: p.target, to: title}
		if msg.err = workspace.ValidPaneTitle(title); msg.err == nil {
			msg.err = c.SetPaneTitle(p.target, title)
		}
		return msg
	}
}

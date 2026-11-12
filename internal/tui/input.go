package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "alt+s" || s == "ctrl+\\" {
		if m.focus == focusTerm {
			m.focus = focusSidebar
		} else if m.term != nil {
			m.focus = focusTerm
		}
		return m, nil
	}
	if m.focus == focusTerm && m.term != nil {
		return m.toTerm(k)
	}
	return m.onSidebarKey(s)
}

// toTerm forwards a key to the embedded client. alt+h is the only key kept
// back: it moves focus to the sidebar when the tmux pane is at the left edge.
func (m model) toTerm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "alt+h" {
		return m, m.edgeCmd()
	}
	if k.Paste {
		m.term.Paste(string(k.Runes))
		return m, nil
	}
	if b, ok := keyBytes(k); ok {
		_ = m.term.Write(b)
	}
	return m, nil
}

func (m model) onSidebarKey(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "ctrl+c", "q":
		if m.term != nil {
			m.term.Close()
		}
		return m, tea.Quit
	case "j", "down":
		m.cursor = min(m.cursor+1, max(0, len(m.rows)-1))
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = max(0, len(m.rows)-1)
	case "r":
		return m, m.refresh()
	case "l", "right", "tab":
		if m.term != nil {
			m.focus = focusTerm
		}
	case "enter":
		return m, m.selectCmd()
	}
	return m, nil
}

package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// onKey routes a key and, when Config.KeyLog is set, records what it did.
func (m model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	before := m.focus
	next, cmd := m.dispatchKey(k)
	m.logKey(k, before, next)
	return next, cmd
}

// logKey writes one line for a key that is not plain typing. Text keys are
// left out on purpose: a password typed into an ssh pane must not reach a
// log. It exists to show whether a key press reaches vantage at all.
func (m model) logKey(k tea.KeyMsg, before focus, next tea.Model) {
	w := m.cfg.KeyLog
	if w == nil || (k.Type == tea.KeyRunes && !k.Alt) || k.Type == tea.KeySpace {
		return
	}
	after := before
	if nm, ok := next.(model); ok {
		after = nm.focus
	}
	name := func(f focus) string {
		if f == focusTerm {
			return "tmux"
		}
		return "sidebar"
	}
	_, _ = fmt.Fprintf(w, "%s key=%q focus=%s->%s\n", time.Now().Format("15:04:05.000"), k.String(), name(before), name(after))
}

func (m model) dispatchKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt != nil {
		return m.onPromptKey(k)
	}
	if m.form != nil {
		return m.onFormKey(k)
	}
	if m.pform != nil {
		return m.onProjFormKey(k)
	}
	if m.projDel != nil {
		return m.onProjDelKey(k)
	}
	if m.hostDel != nil {
		return m.onHostDelKey(k)
	}
	if m.hostPick != nil {
		return m.onHostPickKey(k)
	}
	if m.unlock != nil {
		return m.onUnlockKey(k)
	}
	if m.hostShut != nil {
		return m.onHostShutKey(k)
	}
	if m.help {
		return m.onHelpKey(k)
	}
	s := k.String()
	if s == "alt+/" || s == "alt+?" { // help from either side, like alt+s
		m.help = true
		return m, nil
	}
	if s == "alt+s" && m.view != viewProjects && m.view != viewHosts { // the details pages has no tmux to focus
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
	if m.view == viewHosts {
		return m.onHostKey(s)
	}
	if m.view == viewProjects {
		return m.onProjectKey(s)
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

// promptKeys maps the workspace keys to the question each one opens.
var promptKeys = map[string]promptKind{
	"n": promptNew, "R": promptRename, "d": promptDelete, "K": promptStop, "S": promptSaveAll,
}

func (m model) onSidebarKey(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "ctrl+c", "q":
		if m.term != nil {
			m.term.Close()
		}
		return m, tea.Quit
	case "j", "down":
		m = m.move(1)
	case "k", "up":
		m = m.move(-1)
	case "g", "home":
		m = m.setPos(m.items(), 0)
	case "G", "end":
		m = m.setPos(m.items(), len(m.items())-1)
	case " ":
		m = m.toggle(false)
	case "h", "left":
		m = m.toggle(true)
	case "r":
		return m, m.refresh()
	case "n", "R", "d", "K", "S":
		m = m.startPrompt(promptKeys[s])
	case "H":
		return m.toggleHosts()
	case "P":
		return m.toggleProjects()
	case "?":
		m.help = true
	case "l", "right":
		return m.descend()
	case "tab", "alt+l":
		if m.term != nil {
			m.focus = focusTerm
		}
	case "enter":
		return m, m.selectCmd()
	}
	return m, nil
}

// descend is l: it opens a closed session, steps into an open one, and opens
// a window. Lines with no tree under them just focus tmux.
func (m model) descend() (tea.Model, tea.Cmd) {
	it, ok := m.selected()
	if ok && it.Win >= 0 {
		return m, m.selectCmd()
	}
	if ok {
		if r := m.rows[it.Row]; r.Live && len(r.Wins) > 0 {
			if !m.expanded(r) {
				return m.toggle(false), nil
			}
			return m.move(1), nil
		}
	}
	if m.term != nil {
		m.focus = focusTerm
	}
	return m, nil
}

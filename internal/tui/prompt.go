package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type promptKind int

const (
	promptNew promptKind = iota
	promptRename
	promptDelete
)

// prompt is the one-line input at the bottom of the sidebar. While it is
// open every key goes to it.
type prompt struct {
	kind   promptKind
	target string // session or workspace the prompt acts on
	text   string // typed text (new, rename)
}

// label is the text before the cursor.
func (p prompt) label() string {
	switch p.kind {
	case promptNew:
		return "new: "
	case promptRename:
		return "rename: "
	}
	return "delete " + p.target + "? y/n"
}

// line draws the prompt on one sidebar line, keeping the end of long text
// visible next to the cursor.
func (p prompt) line() string {
	if p.kind == promptDelete {
		return fit(noticeStyle.Render(p.label()), sidebarW)
	}
	room := max(1, sidebarW-len(p.label())-1)
	text := p.text
	if r := []rune(text); len(r) > room {
		text = "…" + string(r[len(r)-room+1:])
	}
	return fit(p.label()+text+cursorStyle.Render(" "), sidebarW)
}

// startPrompt opens a prompt for the line under the cursor, or says why not.
func (m model) startPrompt(kind promptKind) model {
	if m.cfg.Manage == nil {
		return m
	}
	if kind == promptNew {
		m.prompt, m.notice = &prompt{kind: kind}, ""
		return m
	}
	it, ok := m.selected()
	if !ok {
		return m
	}
	r := m.rows[it.Row]
	m.prompt, m.notice = &prompt{kind: kind, target: r.Name, text: r.Name}, ""
	return m
}

func (m model) onPromptKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := *m.prompt
	m.prompt = &p
	if p.kind == promptDelete {
		m.prompt = nil
		if k.String() == "y" {
			return m, m.manageCmd(p)
		}
		return m, nil
	}
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.prompt = nil
	case tea.KeyEnter:
		m.prompt = nil
		if strings.TrimSpace(p.text) == "" || p.text == p.target {
			return m, nil
		}
		return m, m.manageCmd(p)
	case tea.KeyBackspace:
		if r := []rune(p.text); len(r) > 0 {
			m.prompt.text = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		m.prompt.text = ""
	case tea.KeyRunes:
		m.prompt.text += strings.ReplaceAll(strings.ReplaceAll(string(k.Runes), "\n", ""), "\r", "")
	case tea.KeySpace:
		m.prompt.text += " "
	}
	return m, nil
}

// manageCmd runs the confirmed prompt in the background.
func (m model) manageCmd(p prompt) tea.Cmd {
	mg, name := m.cfg.Manage, strings.TrimSpace(p.text)
	return func() tea.Msg {
		msg := manageMsg{op: p.kind, from: p.target, to: name}
		switch p.kind {
		case promptNew:
			msg.err = mg.Create(name)
		case promptRename:
			msg.err = mg.Rename(p.target, name)
		case promptDelete:
			msg.err = mg.Delete(p.target)
		}
		return msg
	}
}

// onManage applies a finished create, rename or delete.
func (m model) onManage(msg manageMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = ansi.Strip(msg.err.Error())
		return m, m.refresh()
	}
	m.notice = ""
	switch msg.op {
	case promptNew:
		m.cur = key{msg.to, -1}
		return m.onReady(readyMsg{name: msg.to})
	case promptRename:
		m = m.renamed(msg.from, msg.to)
	case promptDelete:
		m = m.afterDelete(msg.from)
	}
	return m, m.refresh()
}

// renamed moves everything the model keeps by session name over to the new
// name, so the cursor, the open state and the bold current marker follow.
func (m model) renamed(from, to string) model {
	if m.cur.Session == from {
		m.cur.Session = to
	}
	if m.current == from {
		m.current = to
	}
	if v, ok := m.open[from]; ok {
		next := map[string]bool{to: v}
		for k, o := range m.open {
			if k != from {
				next[k] = o
			}
		}
		m.open = next
	}
	return m
}

// afterDelete parks the cursor on a neighbour: the deleted workspace leaves
// no row behind, and the session it showed is gone.
func (m model) afterDelete(name string) model {
	if m.current == name {
		m.current = ""
	}
	items := m.items()
	if len(items) == 0 {
		return m
	}
	i := m.pos(items)
	it := items[i]
	if m.rows[it.Row].Name != name {
		return m
	}
	for j := i + 1; j < len(items); j++ {
		if items[j].Row != it.Row {
			m.cur = m.keyOf(items[j])
			return m
		}
	}
	for j := i - 1; j >= 0; j-- {
		if items[j].Row != it.Row {
			m.cur = m.keyOf(items[j])
			return m
		}
	}
	return m
}

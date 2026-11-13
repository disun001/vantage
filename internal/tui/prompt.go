package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type promptKind int

const (
	promptNew promptKind = iota
	promptRename
	promptDelete
	promptStop
	promptRenameWin
)

// prompt is the open question about a workspace: its name for new and
// rename, a yes for delete and stop. It shows as a pop-up and owns every key while open.
type prompt struct {
	kind   promptKind
	target string // session or workspace the prompt acts on
	win    int    // window index (rename window)
	text   string // typed text (new, rename)
}

// promptBox is the pop-up for naming a new workspace or renaming one. The
// end of a long name stays visible next to the cursor.
func (m model) promptBox() []string {
	p := m.prompt
	title, hint := "new workspace", "enter create · esc cancels"
	rows := []string{popTitle(title), ""}
	switch p.kind {
	case promptRename:
		rows[0] = popTitle("rename workspace")
		rows = append(rows, dimStyle.Render("from "+p.target), "")
		hint = "enter rename · esc cancels"
	case promptRenameWin:
		rows[0] = popTitle("rename window")
		rows = append(rows, dimStyle.Render(fmt.Sprintf("in %s, window %d", p.target, p.win)), "")
		hint = "enter rename · esc cancels"
	}
	inner := max(10, min(44, m.w-6))
	const label = "name "
	text := p.text
	if room, r := max(1, inner-len(label)-1), []rune(text); len(r) > room {
		text = "…" + string(r[len(r)-room+1:])
	}
	rows = append(rows, curStyle.Render(label)+text+cursorStyle.Render(" "), "", dimStyle.Render(hint))
	return framed(rows, inner)
}

// deleteBox is the pop-up that asks before a workspace goes: what the delete
// removes, and what running work it ends.
func (m model) deleteBox() []string {
	name := m.prompt.target
	rows := []string{dangerTitle("delete workspace"), "", curStyle.Render(name), ""}
	for _, r := range m.rows {
		if r.Name != name {
			continue
		}
		if r.Saved {
			rows = append(rows, "Removes the saved file.")
		}
		if r.Live {
			rows = append(rows, noticeStyle.Render(fmt.Sprintf("Kills the running session (%d windows, %d panes).", r.Windows, r.Panes)))
		}
	}
	rows = append(rows, "", dimStyle.Render("y delete · any other key cancels"))
	return framed(rows, max(10, min(44, m.w-6)))
}

// stopBox is the pop-up that asks before a session is killed. The saved file
// stays, so the workspace can be restored later.
func (m model) stopBox() []string {
	name := m.prompt.target
	rows := []string{dangerTitle("stop session"), "", curStyle.Render(name), ""}
	for _, r := range m.rows {
		if r.Name == name && r.Live {
			rows = append(rows, noticeStyle.Render(fmt.Sprintf("Kills the running session (%d windows, %d panes).", r.Windows, r.Panes)), "Keeps the saved file.")
		}
	}
	rows = append(rows, "", dimStyle.Render("y stop · any other key cancels"))
	return framed(rows, max(10, min(44, m.w-6)))
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
	if kind == promptRename && it.Win >= 0 {
		w := r.Wins[it.Win]
		m.prompt, m.notice = &prompt{kind: promptRenameWin, target: r.Name, win: w.Index, text: w.Name}, ""
		return m
	}
	if kind == promptStop && (!r.Live || !r.Saved) {
		m.notice = "stop needs a saved, running workspace"
		return m
	}
	m.prompt, m.notice = &prompt{kind: kind, target: r.Name, text: r.Name}, ""
	return m
}

func (m model) onPromptKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := *m.prompt
	m.prompt = &p
	if p.kind == promptDelete || p.kind == promptStop {
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
		if strings.TrimSpace(p.text) == "" || (p.kind == promptRename && p.text == p.target) {
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
		case promptRenameWin:
			msg.err = mg.RenameWindow(p.target, p.win, p.text)
		case promptDelete:
			msg.err = mg.Delete(p.target)
		case promptStop:
			msg.err = mg.Stop(p.target)
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
	case promptRenameWin: // the next refresh shows the new name
	case promptDelete:
		m = m.afterDelete(msg.from)
	case promptStop:
		if m.current == msg.from {
			m.current = ""
		}
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

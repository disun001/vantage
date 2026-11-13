package tui

import (
	"sort"

	"github.com/disun001/vantage/internal/tmux"
)

// win is one window under a live session.
type win struct {
	Index  int
	Name   string
	Panes  int
	Active bool
	Chat   string // status of the most urgent chat in the window, or ""
}

// row is one sidebar entry: a live session, a saved workspace that is not
// running, or both (live wins). Live sessions carry their windows.
type row struct {
	Name    string
	Live    bool
	Saved   bool // a workspace file exists
	Windows int
	Panes   int
	Wins    []win
	Chat    string // most urgent chat status in the session, or ""
}

type winKey struct {
	Session string
	Index   int
}

// chatRank orders chat statuses by how much they need a look: a failed or
// cut-off chat first, then a busy one, then an idle one.
var chatRank = map[string]int{"error": 4, "interrupted": 3, "running": 2, "idle": 1}

// moreUrgent returns the status that needs attention more.
func moreUrgent(a, b string) string {
	if chatRank[b] > chatRank[a] {
		return b
	}
	return a
}

// item locates one visible sidebar line: the session line of rows[Row] when
// Win is -1, else its window rows[Row].Wins[Win].
type item struct{ Row, Win int }

// key identifies an item by name, not position, so the cursor stays on the
// same session or window when lines appear or vanish above it.
type key struct {
	Session string
	Win     int // window index, or -1 for the session line
}

// mergeRows combines live sessions and saved workspace names. Live sessions
// come first in tmux order, then saved-only workspaces sorted by name.
// chats maps a live pane id to the status of the chat running in it.
func mergeRows(live []tmux.Session, wins []tmux.Window, panes []tmux.PaneRef, saved []string, chats map[string]string) []row {
	count := map[string]int{}
	winChat := map[winKey]string{}
	sessChat := map[string]string{}
	for _, p := range panes {
		count[p.Session]++
		if st, ok := chats[p.Pane]; ok {
			k := winKey{p.Session, p.Window}
			winChat[k] = moreUrgent(winChat[k], st)
			sessChat[p.Session] = moreUrgent(sessChat[p.Session], st)
		}
	}
	byID := map[string][]win{}
	for _, w := range wins {
		byID[w.SessionID] = append(byID[w.SessionID], win{Index: w.Index, Name: w.Name, Panes: w.Panes, Active: w.Active})
	}
	rows := make([]row, 0, len(live)+len(saved))
	seen := map[string]bool{}
	isSaved := map[string]bool{}
	for _, n := range saved {
		isSaved[n] = true
	}
	for _, s := range live {
		ws := byID[s.ID]
		for i := range ws {
			ws[i].Chat = winChat[winKey{s.Name, ws[i].Index}]
		}
		rows = append(rows, row{Name: s.Name, Live: true, Saved: isSaved[s.Name], Windows: s.Windows, Panes: count[s.Name], Wins: ws, Chat: sessChat[s.Name]})
		seen[s.Name] = true
	}
	var rest []string
	for _, n := range saved {
		if !seen[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	for _, n := range rest {
		rows = append(rows, row{Name: n, Saved: true})
	}
	return rows
}

// expanded reports whether a session shows its windows. Until the user
// toggles it, a session is open exactly while it is the one on screen.
func (m model) expanded(r row) bool {
	if v, ok := m.open[r.Name]; ok {
		return v
	}
	return r.Name == m.current
}

// items flattens the rows into the visible lines.
func (m model) items() []item {
	var out []item
	for i, r := range m.rows {
		out = append(out, item{i, -1})
		if r.Live && m.expanded(r) {
			for j := range r.Wins {
				out = append(out, item{i, j})
			}
		}
	}
	return out
}

func (m model) keyOf(it item) key {
	r := m.rows[it.Row]
	if it.Win < 0 {
		return key{r.Name, -1}
	}
	return key{r.Name, r.Wins[it.Win].Index}
}

// pos is the index of the cursor in items. A cursor whose window is gone
// falls back to its session line, and to the top when that is gone too.
func (m model) pos(items []item) int {
	fallback := 0
	for i, it := range items {
		k := m.keyOf(it)
		if k == m.cur {
			return i
		}
		if k.Win < 0 && k.Session == m.cur.Session {
			fallback = i
		}
	}
	return fallback
}

// setPos moves the cursor to line i, clamped to the list.
func (m model) setPos(items []item, i int) model {
	if len(items) == 0 {
		return m
	}
	m.cur = m.keyOf(items[max(0, min(i, len(items)-1))])
	return m
}

func (m model) move(delta int) model {
	items := m.items()
	return m.setPos(items, m.pos(items)+delta)
}

// selected is the item under the cursor.
func (m model) selected() (item, bool) {
	items := m.items()
	if len(items) == 0 {
		return item{}, false
	}
	return items[m.pos(items)], true
}

// listCap is how many sidebar lines fit between the title and the footer.
func (m model) listCap() int {
	return max(1, m.h-firstRowY-1-len(m.noticeLines()))
}

// scrolled keeps the cursor line inside the visible window of the list.
func (m model) scrolled() model {
	if m.view == viewHosts {
		return m.hostScrolled()
	}
	if m.view == viewProjects {
		return m.projScrolled()
	}
	items := m.items()
	c, p := m.listCap(), m.pos(items)
	if p < m.top {
		m.top = p
	}
	if p >= m.top+c {
		m.top = p - c + 1
	}
	m.top = max(0, min(m.top, len(items)-c))
	return m
}

// toggle opens or closes the session under the cursor. A window line toggles
// its session. Saved workspaces have nothing to open.
func (m model) toggle(collapseOnly bool) model {
	it, ok := m.selected()
	if !ok || !m.rows[it.Row].Live {
		return m
	}
	r := m.rows[it.Row]
	if collapseOnly && !m.expanded(r) {
		return m
	}
	if m.open == nil {
		m.open = map[string]bool{}
	}
	m.open[r.Name] = !m.expanded(r)
	m.cur = key{r.Name, -1}
	return m
}

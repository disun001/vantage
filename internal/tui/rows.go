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
func mergeRows(live []tmux.Session, wins []tmux.Window, panes []tmux.PaneRef, saved []string) []row {
	count := map[string]int{}
	for _, p := range panes {
		count[p.Session]++
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
		rows = append(rows, row{Name: s.Name, Live: true, Saved: isSaved[s.Name], Windows: s.Windows, Panes: count[s.Name], Wins: byID[s.ID]})
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

// expanded reports whether a session shows its windows: every session does
// until the user folds it.
func (m model) expanded(r row) bool { return !m.folded[r.Name] }

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
	return max(1, m.h-firstRowY-1-len(m.noticeLines())-len(m.limitRows()))
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
	m.top = scrollTop(m.headItems(items), m.top, m.pos(items), m.listCap())
	return m
}

// toggle folds or unfolds the session under the cursor and remembers it. A
// window line toggles its session. Saved workspaces have nothing to fold.
func (m model) toggle(collapseOnly bool) model {
	it, ok := m.selected()
	if !ok || !m.rows[it.Row].Live {
		return m
	}
	r := m.rows[it.Row]
	if collapseOnly && !m.expanded(r) {
		return m
	}
	m.folded = withFold(m.folded, r.Name, m.expanded(r))
	m.cur = key{r.Name, -1}
	return m.saveFolds()
}

// withFold returns a copy of folded with name set or cleared. The model is
// copied by value, so the map is never edited in place.
func withFold(folded map[string]bool, name string, fold bool) map[string]bool {
	next := make(map[string]bool, len(folded)+1)
	for k := range folded {
		next[k] = true
	}
	if fold {
		next[name] = true
	} else {
		delete(next, name)
	}
	return next
}

// pruneFolds forgets folded names that are neither a live session nor a saved
// workspace, so a new session that reuses an old name starts unfolded.
func (m model) pruneFolds() model {
	known := map[string]bool{}
	for _, r := range m.rows {
		known[r.Name] = true
	}
	next := map[string]bool{}
	for n := range m.folded {
		if known[n] {
			next[n] = true
		}
	}
	if len(next) == len(m.folded) {
		return m
	}
	m.folded = next
	return m.saveFolds()
}

// saveFolds stores the folded names, sorted. A failure shows as a notice and
// the fold still holds until vantage quits.
func (m model) saveFolds() model {
	if m.cfg.SaveFolds == nil {
		return m
	}
	names := make([]string, 0, len(m.folded))
	for n := range m.folded {
		names = append(names, n)
	}
	sort.Strings(names)
	if err := m.cfg.SaveFolds(names); err != nil {
		m.notice = err.Error()
	}
	return m
}

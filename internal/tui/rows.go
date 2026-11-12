package tui

import (
	"sort"

	"github.com/disun001/vantage/internal/tmux"
)

// row is one sidebar entry: a live session, a saved workspace that is not
// running, or both (live wins).
type row struct {
	Name    string
	Live    bool
	Windows int
	Panes   int
}

// mergeRows combines live sessions and saved workspace names. Live sessions
// come first in tmux order, then saved-only workspaces sorted by name.
func mergeRows(live []tmux.Session, panes []tmux.PaneRef, saved []string) []row {
	count := map[string]int{}
	for _, p := range panes {
		count[p.Session]++
	}
	rows := make([]row, 0, len(live)+len(saved))
	seen := map[string]bool{}
	for _, s := range live {
		rows = append(rows, row{Name: s.Name, Live: true, Windows: s.Windows, Panes: count[s.Name]})
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
		rows = append(rows, row{Name: n})
	}
	return rows
}

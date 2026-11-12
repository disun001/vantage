// Package connections finds which ssh hosts are already open in live tmux
// panes. It only reports; it never changes any pane.
package connections

import (
	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/tmux"
)

// Find maps each ssh alias to the panes currently running exactly
// `ssh <alias>` in the foreground, in tmux's own pane order. Only plain alias
// connections are detected, the same rule capture uses. A pane whose process
// list cannot be read is skipped, so one odd pane never hides the others.
func Find(c tmux.Client, procs capture.Lister) (map[string][]tmux.PaneRef, error) {
	panes, err := c.ListAllPanes()
	if err != nil {
		return nil, err
	}
	return find(panes, procs), nil
}

func find(panes []tmux.PaneRef, procs capture.Lister) map[string][]tmux.PaneRef {
	out := map[string][]tmux.PaneRef{}
	for _, p := range panes {
		lines, err := procs(p.TTY)
		if err != nil {
			continue
		}
		for _, l := range lines {
			if alias, ok := capture.SSHAlias(l); ok {
				out[alias] = append(out[alias], p)
				break
			}
		}
	}
	return out
}

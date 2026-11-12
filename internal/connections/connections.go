// Package connections finds which ssh hosts are already open in live tmux
// panes. It only reports; it never changes any pane.
package connections

import (
	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/tmux"
)

// Find maps each ssh destination (the host word of the command, so an alias,
// a hostname, or an IP) to the panes currently running ssh to it in the
// foreground, in tmux's own pane order. Options like -p and user@ are
// ignored. A pane whose process list cannot be read is skipped, so one odd
// pane never hides the others. Use Lookup to match a configured host.
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
			if dest, ok := capture.SSHDestination(l); ok {
				out[dest] = append(out[dest], p)
				break
			}
		}
	}
	return out
}

// Lookup returns the panes connected to any of the given names (a host's
// alias and its real HostName), without listing a pane twice.
func Lookup(open map[string][]tmux.PaneRef, names ...string) []tmux.PaneRef {
	var out []tmux.PaneRef
	seen := map[string]bool{}
	for _, n := range names {
		if n == "" {
			continue
		}
		for _, p := range open[n] {
			if !seen[p.Pane] {
				seen[p.Pane] = true
				out = append(out, p)
			}
		}
	}
	return out
}

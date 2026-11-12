// Package hostuse finds what depends on an ssh host alias before it is
// deleted: saved workspaces with a pane for it and live tmux panes connected
// to it. It can also close those live panes.
package hostuse

import (
	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/connections"
	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// Usage is what depends on one alias.
type Usage struct {
	// Workspaces are saved workspaces with a pane for the alias.
	Workspaces []string
	// Panes are live panes running ssh to the alias.
	Panes []tmux.PaneRef
}

// Empty reports whether nothing depends on the alias.
func (u Usage) Empty() bool { return len(u.Workspaces) == 0 && len(u.Panes) == 0 }

// Finder looks hosts up in the saved workspaces and the live tmux server.
type Finder struct {
	Client tmux.Client
	Procs  capture.Lister
}

// Find reports the usage of alias. Only the alias counts for live panes:
// `ssh 192.0.2.1` typed by hand keeps working when the alias goes away. Either
// half may fail on its own (no tmux server, one unreadable workspace file), so
// the usage found so far comes back together with the error.
func (f Finder) Find(alias string) (Usage, error) {
	var u Usage
	var first error
	ws, err := workspace.UsingSSH(alias)
	u.Workspaces, first = ws, err
	open, err := connections.Find(f.Client, f.Procs)
	if err != nil && first == nil {
		first = err
	}
	u.Panes = connections.Lookup(open, alias)
	return u, first
}

// Close kills the panes of u. It goes on after a failure and returns the
// first error.
func (f Finder) Close(u Usage) error {
	var first error
	for _, p := range u.Panes {
		if err := f.Client.KillPane(p.Pane); err != nil && first == nil {
			first = err
		}
	}
	return first
}

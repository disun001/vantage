// Package hostuse finds what depends on an ssh host alias before it is
// deleted: saved workspaces with a pane for it and live tmux panes connected
// to it. It can also close those live panes, strip the saved ones and point
// them at a renamed alias.
package hostuse

import (
	"errors"
	"fmt"

	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/connections"
	"github.com/disun001/vantage/internal/sshconfig"
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

// Live returns the open panes running ssh to the alias, in tmux's pane order.
func (f Finder) Live(alias string) ([]tmux.PaneRef, error) {
	open, err := connections.Find(f.Client, f.Procs)
	if err != nil {
		return nil, err
	}
	return connections.Lookup(open, alias), nil
}

// Connected reports which hosts have an open pane running ssh to their alias
// or real host name, for the sidebar's connection dot.
func (f Finder) Connected(hosts []sshconfig.Host) (map[string]bool, error) {
	open, err := connections.Find(f.Client, f.Procs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		out[h.Alias] = len(connections.Lookup(open, h.Alias, h.HostName)) > 0
	}
	return out, nil
}

// Strip removes the panes for alias from the saved workspaces (see
// workspace.StripSSH for what that touches and what it refuses).
func (f Finder) Strip(alias string) error {
	_, err := workspace.StripSSH(alias)
	return err
}

// Rename points the saved workspaces' panes for from at the alias to (see
// workspace.RenameSSH) and tags the open panes running ssh to from, so they
// are still found under the new alias: their process keeps saying `ssh from`.
// Both steps run even if one fails.
func (f Finder) Rename(from, to string) error {
	var errs []error
	if _, err := workspace.RenameSSH(from, to); err != nil {
		errs = append(errs, fmt.Errorf("workspaces not fully updated: %w", err))
	}
	if err := f.retag(from, to); err != nil {
		errs = append(errs, fmt.Errorf("open panes not retagged: %w", err))
	}
	return errors.Join(errs...)
}

// retag records on each open pane of from that its host is called to now.
// The tag keeps the alias the process really runs, so a chain of renames
// still ends at it, and a rename back clears the tag.
func (f Finder) retag(from, to string) error {
	open, err := connections.Find(f.Client, f.Procs)
	if err != nil {
		if tmux.IsNoServer(err) {
			return nil // no server, no panes
		}
		return err
	}
	var first error
	for _, p := range connections.Lookup(open, from) {
		orig := from
		if o, _, ok := connections.ParseRenamed(p.Renamed); ok {
			orig = o
		}
		if orig == to {
			err = f.Client.ClearRenamedTag(p.Pane)
		} else {
			err = f.Client.SetRenamedTag(p.Pane, orig+">"+to)
		}
		if err != nil && first == nil {
			first = err
		}
	}
	return first
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

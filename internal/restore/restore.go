// Package restore builds a live tmux session from a workspace definition.
package restore

import (
	"fmt"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// Result describes what Restore did.
type Result struct {
	// Existed is true when the session was already running; nothing was changed.
	Existed bool
	Windows int
	Panes   int
}

// Restore creates the session for ws on the given tmux server. It is
// idempotent: if a session with the workspace's name already exists it is
// left untouched. If creation fails part way, the half-built session is
// removed so a retry starts clean.
func Restore(c tmux.Client, ws workspace.Workspace) (res Result, err error) {
	if err := ws.Validate(); err != nil {
		return res, err
	}
	if c.HasSession(ws.Name) {
		return Result{Existed: true}, nil
	}
	created := false
	defer func() {
		if err != nil && created {
			_ = c.KillSession(ws.Name)
		}
	}()

	for wi, win := range ws.Windows {
		first := win.Panes[0]
		var pane string
		if wi == 0 {
			pane, err = c.NewSession(ws.Name, win.Name, ws.ResolveCWD(first))
			created = err == nil
		} else {
			pane, err = c.NewWindow(ws.Name, win.Name, ws.ResolveCWD(first))
		}
		if err != nil {
			return res, err
		}
		res.Windows++
		res.Panes++
		if err = start(c, pane, first); err != nil {
			return res, err
		}

		prev := pane
		for _, p := range win.Panes[1:] {
			prev, err = c.SplitWindow(prev, p.Split, ws.ResolveCWD(p))
			if err != nil {
				return res, err
			}
			res.Panes++
			if err = start(c, prev, p); err != nil {
				return res, err
			}
		}
		if win.Layout != "" {
			if err = c.SelectLayout(pane, win.Layout); err != nil {
				return res, fmt.Errorf("window %q layout: %w", win.Name, err)
			}
		}
	}
	return res, nil
}

// start runs what a pane asks for: an ssh connection, or an opted-in command.
func start(c tmux.Client, pane string, p workspace.Pane) error {
	switch {
	case p.SSH != "":
		return c.SendLine(pane, "ssh "+p.SSH)
	case p.Cmd != "" && p.Autorun:
		return c.SendLine(pane, p.Cmd)
	}
	return nil
}

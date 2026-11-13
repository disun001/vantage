package main

import "github.com/disun001/vantage/internal/tmux"

// fallbackSession is created when there is nothing to restore.
const fallbackSession = "main"

// starter is what ensureSession needs from tmux.
type starter interface {
	ListSessions() ([]tmux.Session, error)
	NewSession(name, window, cwd string) (string, error)
}

// ensureSession makes sure the TUI has a session to show. It does nothing
// when a session is live. Otherwise it restores the first saved workspace
// (names are sorted), or creates a bare session when none are saved.
func ensureSession(c starter, saved func() ([]string, error), restore func(string) error, home string) error {
	live, err := c.ListSessions()
	if err != nil {
		return err
	}
	if len(live) > 0 {
		return nil
	}
	names, err := saved()
	if err != nil {
		return err
	}
	if len(names) > 0 {
		return restore(names[0])
	}
	_, err = c.NewSession(fallbackSession, "", home)
	return err
}

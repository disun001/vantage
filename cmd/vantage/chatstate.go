package main

import (
	"context"
	"time"

	"github.com/disun001/vantage/internal/store"
)

// chatStates feeds the sidebar's chat markers from the database. It opens the
// database on first use, and only once the file exists, so people who never
// start a chat never get one created by the sidebar.
type chatStates struct {
	dir, socket string
	st          *store.Store
}

func newChatStates(socket string) (*chatStates, error) {
	dir, err := store.StateDir()
	if err != nil {
		return nil, err
	}
	if socket == "" {
		socket = "default" // the file name tmux gives its default server
	}
	return &chatStates{dir: dir, socket: socket}, nil
}

// PaneStatuses maps pane id to chat status for this tmux server.
func (c *chatStates) PaneStatuses() (map[string]string, error) {
	if c.st == nil {
		if !store.Exists(c.dir) {
			return nil, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		st, err := store.Open(ctx, c.dir)
		if err != nil {
			return nil, err
		}
		c.st = st
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	states, err := c.st.PaneStates(ctx, c.socket)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(states))
	for _, s := range states {
		out[s.Pane] = s.Status
	}
	return out, nil
}

// Close releases the database if it was opened.
func (c *chatStates) Close() {
	if c.st != nil {
		_ = c.st.Close()
	}
}

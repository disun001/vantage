package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStateDirPrecedence(t *testing.T) {
	t.Setenv("VANTAGE_STATE_DIR", "/a")
	t.Setenv("XDG_STATE_HOME", "/b")
	if d, _ := StateDir(); d != "/a" {
		t.Fatalf("got %q", d)
	}
	t.Setenv("VANTAGE_STATE_DIR", "")
	if d, _ := StateDir(); d != filepath.Join("/b", "vantage") {
		t.Fatalf("got %q", d)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/h")
	if d, _ := StateDir(); d != filepath.Join("/h", ".local", "state", "vantage") {
		t.Fatalf("got %q", d)
	}
}

func TestOpenPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	di, _ := os.Stat(dir)
	fi, _ := os.Stat(filepath.Join(dir, "vantage.db"))
	if di.Mode().Perm() != 0o700 || fi.Mode().Perm() != 0o600 {
		t.Fatalf("dir %v file %v", di.Mode().Perm(), fi.Mode().Perm())
	}
}

func TestChatAndEvents(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	if _, err := s.Chat(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.CreateChat(ctx, NewChat{ID: "c1", Provider: "claude", Cwd: "/work"}); err != nil {
		t.Fatal(err)
	}
	for i, k := range []string{"user_msg", "assistant_msg"} {
		st := ""
		if i == 0 {
			st = StatusRunning
		}
		if err := s.Append(ctx, NewEvent{ChatID: "c1", Turn: 1, Kind: k, Text: k, Raw: "{}", Status: st}); err != nil {
			t.Fatal(err)
		}
	}
	es, err := s.Events(ctx, "c1")
	if err != nil || len(es) != 2 || es[0].Seq != 1 || es[1].Seq != 2 {
		t.Fatalf("events %+v err %v", es, err)
	}
	c, _ := s.Chat(ctx, "c1")
	if c.Status != StatusRunning {
		t.Fatalf("status %q", c.Status)
	}
	if n, _ := s.LastTurn(ctx, "c1"); n != 1 {
		t.Fatalf("last turn %d", n)
	}
	if err := s.SetProviderSession(ctx, "c1", "sess"); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Chat(ctx, "c1")
	if c.ProviderSession.String != "sess" {
		t.Fatalf("session %+v", c.ProviderSession)
	}
}

func TestReopenKeepsData(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "state")
	s, _ := Open(ctx, dir)
	_ = s.CreateChat(ctx, NewChat{ID: "c1", Provider: "claude", Cwd: "/w"})
	_ = s.Close()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.Chat(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
}

func TestPaneStates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if Exists(dir) {
		t.Fatal("Exists true before Open")
	}
	st, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if !Exists(dir) {
		t.Fatal("Exists false after Open")
	}
	for _, c := range []NewChat{
		{ID: "a", Provider: "claude", Cwd: "/x", Pane: "%1", Socket: "default"},
		{ID: "b", Provider: "claude", Cwd: "/x", Pane: "%1", Socket: "other"},
		{ID: "c", Provider: "claude", Cwd: "/x", Socket: "default"}, // no pane
	} {
		if err := st.CreateChat(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetStatus(ctx, "a", StatusRunning); err != nil {
		t.Fatal(err)
	}
	got, err := st.PaneStates(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (PaneState{Pane: "%1", Status: StatusRunning}) {
		t.Fatalf("PaneStates = %+v, want only chat a", got)
	}
}

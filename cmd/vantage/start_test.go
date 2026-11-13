package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
)

type fakeStarter struct {
	live    []tmux.Session
	listErr error
	created []string
}

func (f *fakeStarter) ListSessions() ([]tmux.Session, error) { return f.live, f.listErr }
func (f *fakeStarter) NewSession(name, _, _ string) (string, error) {
	f.created = append(f.created, name)
	return "%1", nil
}

func TestEnsureSession(t *testing.T) {
	saved := func(names ...string) func() ([]string, error) {
		return func() ([]string, error) { return names, nil }
	}
	t.Run("live session: nothing to do", func(t *testing.T) {
		f := &fakeStarter{live: []tmux.Session{{Name: "x"}}}
		var restored []string
		err := ensureSession(f, saved("a"), func(n string) error { restored = append(restored, n); return nil }, "/h")
		if err != nil || len(restored) != 0 || len(f.created) != 0 {
			t.Errorf("err=%v restored=%v created=%v", err, restored, f.created)
		}
	})
	t.Run("restores the first saved workspace", func(t *testing.T) {
		f := &fakeStarter{}
		var restored []string
		err := ensureSession(f, saved("alpha", "beta"), func(n string) error { restored = append(restored, n); return nil }, "/h")
		if err != nil || len(restored) != 1 || restored[0] != "alpha" || len(f.created) != 0 {
			t.Errorf("err=%v restored=%v created=%v", err, restored, f.created)
		}
	})
	t.Run("none saved: bare main", func(t *testing.T) {
		f := &fakeStarter{}
		err := ensureSession(f, saved(), func(string) error { t.Error("restore called"); return nil }, "/h")
		if err != nil || len(f.created) != 1 || f.created[0] != "main" {
			t.Errorf("err=%v created=%v", err, f.created)
		}
	})
	t.Run("errors are returned", func(t *testing.T) {
		boom := errors.New("boom")
		if err := ensureSession(&fakeStarter{listErr: boom}, saved(), nil, "/h"); !errors.Is(err, boom) {
			t.Errorf("list: %v", err)
		}
		if err := ensureSession(&fakeStarter{}, func() ([]string, error) { return nil, boom }, nil, "/h"); !errors.Is(err, boom) {
			t.Errorf("saved: %v", err)
		}
		if err := ensureSession(&fakeStarter{}, saved("a"), func(string) error { return boom }, "/h"); !errors.Is(err, boom) {
			t.Errorf("restore: %v", err)
		}
	})
}

func TestEnsureSessionOnARealServer(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	none := func() ([]string, error) { return nil, nil }
	if err := ensureSession(c, none, nil, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !c.HasSession("main") {
		t.Error("no main session on a cold server")
	}
	// A second call sees the live session and changes nothing.
	if err := ensureSession(c, none, nil, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

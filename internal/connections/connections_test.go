package connections

import (
	"errors"
	"reflect"
	"testing"

	"github.com/disun001/vantage/internal/tmux"
)

func TestFind(t *testing.T) {
	panes := []tmux.PaneRef{
		{Session: "a", Window: 1, Pane: "%0", TTY: "/dev/t0"},
		{Session: "a", Window: 2, Pane: "%1", TTY: "/dev/t1"},
		{Session: "b", Window: 1, Pane: "%2", TTY: "/dev/t2"},
		{Session: "b", Window: 2, Pane: "%3", TTY: "/dev/t3"},
		{Session: "b", Window: 3, Pane: "%4", TTY: "/dev/t4"},
	}
	procs := map[string][]string{
		"/dev/t0": {"-zsh"},
		"/dev/t1": {"zsh -c ssh web; rc=$?; exit $rc", "ssh web"},
		"/dev/t2": {"ssh web"},
		"/dev/t3": {"ssh -p 2200 me@db"}, // options and user are ignored
		"/dev/t4": nil,
	}
	lister := func(tty string) ([]string, error) {
		if tty == "/dev/t4" {
			return nil, errors.New("ps failed")
		}
		return procs[tty], nil
	}
	got := find(panes, lister)
	want := map[string][]tmux.PaneRef{"web": {panes[1], panes[2]}, "db": {panes[3]}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLookup(t *testing.T) {
	p1, p2 := tmux.PaneRef{Pane: "%1"}, tmux.PaneRef{Pane: "%2"}
	open := map[string][]tmux.PaneRef{"web": {p1}, "10.0.0.5": {p1, p2}}
	got := Lookup(open, "web", "10.0.0.5", "")
	if !reflect.DeepEqual(got, []tmux.PaneRef{p1, p2}) {
		t.Fatalf("got %v, want %v then %v once each", got, p1, p2)
	}
	if got := Lookup(open, "nope"); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

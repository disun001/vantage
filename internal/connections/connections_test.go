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
		"/dev/t3": {"ssh -p 2200 db"}, // options: not a plain alias
		"/dev/t4": nil,
	}
	lister := func(tty string) ([]string, error) {
		if tty == "/dev/t4" {
			return nil, errors.New("ps failed")
		}
		return procs[tty], nil
	}
	got := find(panes, lister)
	want := map[string][]tmux.PaneRef{"web": {panes[1], panes[2]}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

package tui

import (
	"reflect"
	"testing"

	"github.com/disun001/vantage/internal/tmux"
)

func TestMergeRows(t *testing.T) {
	live := []tmux.Session{{Name: "b", Windows: 2}, {Name: "a", Windows: 1}}
	panes := []tmux.PaneRef{{Session: "b"}, {Session: "b"}, {Session: "a"}}
	got := mergeRows(live, panes, []string{"z", "a", "c"})
	want := []row{
		{Name: "b", Live: true, Windows: 2, Panes: 2},
		{Name: "a", Live: true, Windows: 1, Panes: 1},
		{Name: "c"},
		{Name: "z"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

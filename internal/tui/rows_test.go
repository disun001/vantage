package tui

import (
	"reflect"
	"testing"

	"github.com/disun001/vantage/internal/tmux"
)

func TestMergeRows(t *testing.T) {
	live := []tmux.Session{{ID: "$1", Name: "b", Windows: 2}, {ID: "$0", Name: "a", Windows: 1}}
	wins := []tmux.Window{
		{SessionID: "$1", Index: 1, Name: "x", Panes: 2, Active: true},
		{SessionID: "$1", Index: 2, Name: "y", Panes: 1},
		{SessionID: "$0", Index: 0, Name: "z", Panes: 1, Active: true},
	}
	panes := []tmux.PaneRef{{Session: "b"}, {Session: "b"}, {Session: "a"}}
	got := mergeRows(live, wins, panes, []string{"z", "a", "c"})
	want := []row{
		{Name: "b", Live: true, Windows: 2, Panes: 2, Wins: []win{{1, "x", 2, true}, {2, "y", 1, false}}},
		{Name: "a", Live: true, Saved: true, Windows: 1, Panes: 1, Wins: []win{{0, "z", 1, true}}},
		{Name: "c", Saved: true},
		{Name: "z", Saved: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func treeModel() model {
	return model{
		w: 60, h: 20,
		rows: []row{
			{Name: "a", Live: true, Wins: []win{{1, "sh", 1, true}, {2, "vim", 2, false}}},
			{Name: "b", Live: true, Wins: []win{{1, "ssh", 1, true}}},
			{Name: "saved"},
		},
		current: "a",
	}
}

func TestItemsExpandCurrentOnly(t *testing.T) {
	m := treeModel()
	want := []item{{0, -1}, {0, 0}, {0, 1}, {1, -1}, {2, -1}}
	if got := m.items(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	m.current = "b"
	want = []item{{0, -1}, {1, -1}, {1, 0}, {2, -1}}
	if got := m.items(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after switching: got %v, want %v", got, want)
	}
}

// The cursor is a name, so it must not slip onto another line when the
// current session changes and the lines above it collapse.
func TestCursorSticksToItsWindow(t *testing.T) {
	m := treeModel()
	m.cur = key{"b", -1}
	m.current = "b" // a collapses, b expands
	if it, _ := m.selected(); it != (item{1, -1}) {
		t.Errorf("cursor moved to %v", it)
	}
	m.cur = key{"b", 1}
	m.rows[1].Wins = nil // the window vanished
	if it, _ := m.selected(); it != (item{1, -1}) {
		t.Errorf("cursor should fall back to its session, got %v", it)
	}
}

func TestMoveAndToggle(t *testing.T) {
	m := treeModel()
	m = m.move(1).move(1) // a -> a:1 -> a:2
	if m.cur != (key{"a", 2}) {
		t.Fatalf("cur = %v", m.cur)
	}
	m = m.toggle(true) // collapse from a window line: back on the session
	if m.cur != (key{"a", -1}) || len(m.items()) != 3 {
		t.Errorf("collapse: cur=%v items=%v", m.cur, m.items())
	}
	if m = m.toggle(true); len(m.items()) != 3 {
		t.Errorf("collapse of a closed session must do nothing")
	}
	if m = m.toggle(false); len(m.items()) != 5 {
		t.Errorf("toggle did not reopen: %v", m.items())
	}
	m.cur = key{"saved", -1}
	if got := m.toggle(false); !reflect.DeepEqual(got.open, m.open) {
		t.Errorf("saved workspace toggled")
	}
	if m = m.move(5); m.cur != (key{"saved", -1}) {
		t.Errorf("move past the end: %v", m.cur)
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	m := model{w: 60, h: 8} // room for 5 lines
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		m.rows = append(m.rows, row{Name: n})
	}
	if c := m.listCap(); c != 5 {
		t.Fatalf("cap %d", c)
	}
	m = m.setPos(m.items(), 7).scrolled()
	if m.top != 3 {
		t.Errorf("top = %d, want 3", m.top)
	}
	m = m.setPos(m.items(), 4).scrolled()
	if m.top != 3 { // still visible: no jump
		t.Errorf("top = %d after moving up inside the view", m.top)
	}
	m = m.setPos(m.items(), 1).scrolled()
	if m.top != 1 {
		t.Errorf("top = %d, want 1", m.top)
	}
}

func TestDescend(t *testing.T) {
	m := treeModel() // cursor falls back to line a, which is open
	step := func(m model) (model, bool) {
		got, cmd := m.descend()
		return got.(model), cmd != nil
	}

	m, selected := step(m) // open session: step into its first window
	if m.cur != (key{"a", 1}) || selected {
		t.Fatalf("open session: cur=%v selected=%v", m.cur, selected)
	}
	if _, selected = step(m); !selected {
		t.Error("l on a window must open it")
	}

	m.cur = key{"a", -1}
	m = m.toggle(false) // close it
	if len(m.items()) != 3 {
		t.Fatalf("setup: %v", m.items())
	}
	m, selected = step(m) // closed session: open the tree, nothing selected
	if len(m.items()) != 5 || m.cur != (key{"a", -1}) || selected {
		t.Errorf("closed session: items=%v cur=%v selected=%v", m.items(), m.cur, selected)
	}

	m.cur = key{"saved", -1} // nothing to open under a saved workspace
	if got, selected := step(m); got.cur != m.cur || selected {
		t.Errorf("saved: cur=%v selected=%v", got.cur, selected)
	}
}

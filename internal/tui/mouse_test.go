package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestSidebarClickSelectsRow(t *testing.T) {
	m := model{rows: []row{{Name: "a"}, {Name: "b"}, {Name: "c"}}, focus: focusTerm}
	click := tea.MouseMsg{X: 3, Y: firstRowY + 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	got, cmd := m.onMouse(click)
	gm := got.(model)
	if gm.cur != (key{"c", -1}) || gm.focus != focusSidebar || cmd == nil {
		t.Errorf("cur=%v focus=%v cmd=%v", gm.cur, gm.focus, cmd)
	}

	got, _ = gm.onMouse(tea.MouseMsg{X: 3, Y: 5, Button: tea.MouseButtonWheelUp})
	if got.(model).cur != (key{"b", -1}) {
		t.Errorf("wheel up did not move the cursor")
	}
}

func TestToVT(t *testing.T) {
	press := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	ev, ok := toVT(press, 4, 7)
	if c, isClick := ev.(uv.MouseClickEvent); !ok || !isClick || c.X != 4 || c.Y != 7 {
		t.Errorf("press: %#v %v", ev, ok)
	}
	if _, ok := toVT(press, -1, 0); ok {
		t.Error("negative x accepted")
	}
	rel := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}
	if ev, _ := toVT(rel, 0, 0); ev == nil {
		t.Error("release dropped")
	} else if _, isRel := ev.(uv.MouseReleaseEvent); !isRel {
		t.Errorf("release became %T", ev)
	}
	wheel := tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}
	if ev, _ := toVT(wheel, 0, 0); ev == nil {
		t.Error("wheel dropped")
	} else if _, isW := ev.(uv.MouseWheelEvent); !isW {
		t.Errorf("wheel became %T", ev)
	}
}

func TestClickOnArrowFoldsWithoutOpening(t *testing.T) {
	m := treeModel() // a is open: lines a, a:1, a:2, b, saved
	got, cmd := m.onMouse(tea.MouseMsg{X: 0, Y: firstRowY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	gm := got.(model)
	if cmd != nil || len(gm.items()) != 3 {
		t.Errorf("arrow click: cmd=%v items=%v", cmd, gm.items())
	}
	// Clicking a window line opens it.
	got, cmd = m.onMouse(tea.MouseMsg{X: 5, Y: firstRowY + 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if gm = got.(model); gm.cur != (key{"a", 2}) || cmd == nil {
		t.Errorf("window click: cur=%v cmd=%v", gm.cur, cmd)
	}
}

func TestClickAccountsForScroll(t *testing.T) {
	m := model{w: 60, h: 8, top: 3}
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		m.rows = append(m.rows, row{Name: n})
	}
	got, _ := m.onMouse(tea.MouseMsg{X: 3, Y: firstRowY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got.(model).cur != (key{"d", -1}) {
		t.Errorf("first visible line is d, got %v", got.(model).cur)
	}
}

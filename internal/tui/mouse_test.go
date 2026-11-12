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
	if gm.cursor != 2 || gm.focus != focusSidebar || cmd == nil {
		t.Errorf("cursor=%d focus=%v cmd=%v", gm.cursor, gm.focus, cmd)
	}

	got, _ = gm.onMouse(tea.MouseMsg{X: 3, Y: 5, Button: tea.MouseButtonWheelUp})
	if got.(model).cursor != 1 {
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

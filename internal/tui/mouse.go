package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
)

// firstRowY is the screen line of the first sidebar row (see sidebar()).
const firstRowY = 2

// chevronW is the width of the fold arrow that starts a live session's line.
const chevronW = 2

// onMouse routes a mouse event by where it landed. Clicks pick the focus;
// the sidebar handles its own clicks and wheel, and events over the tmux
// region are translated to its coordinates and forwarded.
func (m model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.X < sidebarW {
		return m.onSidebarMouse(msg)
	}
	if msg.X == sidebarW || m.term == nil { // divider, or nothing to forward to
		return m, nil
	}
	if msg.Action == tea.MouseActionPress && m.focus != focusTerm {
		m.focus = focusTerm
	}
	y := msg.Y
	if m.bar.top {
		y += m.bar.lines // the cropped status bar sits above the visible rows
	}
	if ev, ok := toVT(msg, msg.X-sidebarW-1, y); ok {
		m.term.Mouse(ev)
	}
	return m, nil
}

func (m model) onSidebarMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		m = m.move(-1)
	case msg.Button == tea.MouseButtonWheelDown:
		m = m.move(1)
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		m.focus = focusSidebar
		items := m.items()
		if i := msg.Y - firstRowY + m.top; msg.Y >= firstRowY && i < len(items) {
			m = m.setPos(items, i)
			if it := items[i]; msg.X < chevronW && it.Win < 0 && m.rows[it.Row].Live {
				return m.toggle(false), nil // a click on the arrow only folds
			}
			return m, m.selectCmd()
		}
	}
	return m, nil
}

// toVT converts a Bubble Tea mouse event to the emulator's, at x, y relative
// to the embedded screen.
func toVT(msg tea.MouseMsg, x, y int) (uv.MouseEvent, bool) {
	if x < 0 || y < 0 {
		return nil, false
	}
	m := uv.Mouse{X: x, Y: y}
	switch msg.Button {
	case tea.MouseButtonLeft:
		m.Button = uv.MouseLeft
	case tea.MouseButtonMiddle:
		m.Button = uv.MouseMiddle
	case tea.MouseButtonRight:
		m.Button = uv.MouseRight
	case tea.MouseButtonWheelUp:
		m.Button = uv.MouseWheelUp
	case tea.MouseButtonWheelDown:
		m.Button = uv.MouseWheelDown
	case tea.MouseButtonNone:
		m.Button = uv.MouseNone
	default:
		return nil, false
	}
	if msg.Alt {
		m.Mod |= uv.ModAlt
	}
	if msg.Ctrl {
		m.Mod |= uv.ModCtrl
	}
	if msg.Shift {
		m.Mod |= uv.ModShift
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
		return uv.MouseWheelEvent(m), true
	case msg.Action == tea.MouseActionRelease:
		return uv.MouseReleaseEvent(m), true
	case msg.Action == tea.MouseActionMotion:
		return uv.MouseMotionEvent(m), true
	}
	return uv.MouseClickEvent(m), true
}

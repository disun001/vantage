package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// trueColor makes lipgloss emit colors for the test and restores the profile.
func trueColor(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

func TestSidebarLinesKeepTheirWidth(t *testing.T) {
	trueColor(t)
	m := model{w: 100, h: 24, current: "a-rather-long-session-name-here"}
	r := row{
		Name: "a-rather-long-session-name-here", Live: true, Windows: 3, Panes: 12,
		Wins: []win{
			{Index: 1, Name: "a-rather-long-window-title", Panes: 2, Active: true},
			{Index: 10, Name: "x", Panes: 1},
		},
	}
	for _, sel := range []bool{false, true} {
		for _, f := range []focus{focusSidebar, focusTerm} {
			m.focus = f
			lines := map[string]string{
				"live":   m.rowLine(r, sel),
				"saved":  m.rowLine(row{Name: "notes"}, sel),
				"window": m.winLine(r, 0, sel),
				"last":   m.winLine(r, 1, sel),
			}
			for name, l := range lines {
				if w := ansi.StringWidth(l); w != sidebarW {
					t.Errorf("%s (selected=%v focus=%v): width %d, want %d: %q", name, sel, f, w, sidebarW, ansi.Strip(l))
				}
			}
		}
	}
}

const (
	yellowRGB = "249;226;175" // #f9e2af, the tmux bar's current window
	mauveRGB  = "203;166;247" // #cba6f7
)

func TestOnlyTheWindowOnScreenIsYellow(t *testing.T) {
	trueColor(t)
	wins := []win{{Index: 1, Name: "edit", Panes: 2, Active: true}, {Index: 2, Name: "server", Panes: 1}}
	shown := row{Name: "api", Live: true, Windows: 2, Panes: 3, Wins: wins}
	other := row{Name: "web", Live: true, Windows: 2, Panes: 3, Wins: wins}
	m := model{focus: focusSidebar, current: "api"}

	if l := m.winLine(shown, 0, false); !strings.Contains(l, yellowRGB) {
		t.Errorf("active window of the session on screen must be yellow: %q", l)
	}
	if l := m.winLine(shown, 1, false); strings.Contains(l, yellowRGB) {
		t.Errorf("an idle window is yellow: %q", l)
	}
	if l := m.winLine(other, 0, false); strings.Contains(l, yellowRGB) {
		t.Errorf("active window of a session that is not on screen is yellow: %q", l)
	}
	idleShown := m.winLine(shown, 1, false)
	idleOther := m.winLine(other, 1, false)
	activeOther := m.winLine(other, 0, false)
	const grey = "108;112;134" // #6c7086
	if !strings.Contains(idleShown, grey) || !strings.Contains(activeOther, grey) || !strings.Contains(idleOther, grey) {
		t.Error("every non-yellow window must use the same grey chip")
	}
	// No yellow anywhere on the session lines, selected or not.
	for _, sel := range []bool{false, true} {
		if l := m.rowLine(shown, sel); strings.Contains(l, yellowRGB) {
			t.Errorf("session line is yellow: %q", l)
		}
	}
}

func TestSessionOnScreenIsAPillAndSavedIsMuted(t *testing.T) {
	trueColor(t)
	m := model{focus: focusSidebar, current: "api"}
	if l := m.rowLine(row{Name: "api", Live: true, Windows: 1, Panes: 1}, false); !strings.Contains(l, mauveRGB) {
		t.Errorf("current session has no pill: %q", l)
	}
	if l := m.rowLine(row{Name: "web", Live: true, Windows: 1, Panes: 1}, false); strings.Contains(l, mauveRGB) {
		t.Errorf("another session has the pill: %q", l)
	}
	cur := ansi.Strip(m.rowLine(row{Name: "api", Live: true, Windows: 1, Panes: 1}, false))
	oth := ansi.Strip(m.rowLine(row{Name: "web", Live: true, Windows: 1, Panes: 1}, false))
	if strings.Index(cur, "api") != strings.Index(oth, "web") {
		t.Errorf("the pill moves the name: %q vs %q", cur, oth)
	}
	if got := ansi.Strip(m.rowLine(row{Name: "notes"}, false)); !strings.Contains(got, "notes") || !strings.HasSuffix(strings.TrimRight(got, " "), "\uf10c") {
		t.Errorf("saved line: %q", got)
	}
}

func TestWindowTilesAlignAndHang(t *testing.T) {
	m := model{focus: focusSidebar}
	r := row{Name: "api", Live: true, Wins: []win{{Index: 1, Name: "edit", Panes: 2}, {Index: 10, Name: "logs", Panes: 1}}}
	a, b := ansi.Strip(m.winLine(r, 0, false)), ansi.Strip(m.winLine(r, 1, false))
	if !strings.HasPrefix(a, "   ├") || !strings.HasPrefix(b, "   └") {
		t.Errorf("tree guides: %q %q", a, b)
	}
	if strings.Index(a, "edit") != strings.Index(b, "logs") {
		t.Errorf("titles must line up when numbers differ in width: %q %q", a, b)
	}
	if !strings.HasSuffix(a, "2p  ") && !strings.HasSuffix(a, "2p ") {
		t.Errorf("pane count at the right edge: %q", a)
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/sshconfig"
)

func TestHostLineShowsConnection(t *testing.T) {
	m := model{focus: focusSidebar, hostOn: map[string]bool{"web": true}}
	on := ansi.Strip(m.hostLine(sshconfig.Host{Alias: "web"}, false))
	off := ansi.Strip(m.hostLine(sshconfig.Host{Alias: "db"}, false))
	if !strings.HasSuffix(strings.TrimRight(on, " "), iconDot) {
		t.Errorf("connected host has no dot on the right: %q", on)
	}
	if !strings.HasSuffix(strings.TrimRight(off, " "), iconRing) {
		t.Errorf("idle host has no ring on the right: %q", off)
	}
	if got := ansi.StringWidth(on); got != sidebarW {
		t.Errorf("host line is %d wide, want %d", got, sidebarW)
	}
}

func TestCountLabelPluralises(t *testing.T) {
	for n, want := range map[int]string{1: "1 repo", 4: "4 repos"} {
		if got := countLabel(n); got != want {
			t.Errorf("countLabel(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestWorkspaceRowKeepsWidthAndNote(t *testing.T) {
	m := model{focus: focusSidebar, current: "api"}
	line := m.rowLine(row{Name: "a-very-long-session-name-that-overflows", Live: true, Windows: 12, Panes: 34}, false)
	if got := ansi.StringWidth(line); got != sidebarW {
		t.Errorf("row is %d wide, want %d: %q", got, sidebarW, ansi.Strip(line))
	}
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "12w 34p") || !strings.HasSuffix(strings.TrimRight(plain, " "), iconDot) {
		t.Errorf("note or dot cut off: %q", plain)
	}
}

func TestSpanPutsAHairlineBetweenEntriesNotWindows(t *testing.T) {
	// two sessions, the first with two windows
	flat := []bool{true, false, false, true}
	got := span(flat, 0, 10)
	want := []int{0, 1, 2, -1, 3}
	if len(got) != len(want) {
		t.Fatalf("span = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("span = %v, want %v", got, want)
		}
	}
	if got := span(flat, 0, 4); len(got) != 3 { // the hairline and the last entry no longer fit
		t.Errorf("tight span = %v", got)
	}
}

func TestScrollTopKeepsTheCursorInView(t *testing.T) {
	flat := allFlat(10) // 2 lines per entry after the first
	top := 0
	for pos := 0; pos < 10; pos++ {
		top = scrollTop(flat, top, pos, 5)
		if !drawn(span(flat, top, 5), pos) {
			t.Fatalf("cursor %d off screen with top %d", pos, top)
		}
	}
	if top = scrollTop(flat, top, 0, 5); top != 0 {
		t.Errorf("scrolling back to the first entry left top at %d", top)
	}
}

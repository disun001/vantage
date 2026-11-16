package tui

import (
	"reflect"
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

func TestSpanDrawsTheHeaderWithItsEntry(t *testing.T) {
	head := []bool{false, false, false, true}
	if got, want := span(head, 0, 10), []int{0, 1, 2, -1, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("span = %v, want %v", got, want)
	}
	if got := span(head, 0, 4); !reflect.DeepEqual(got, []int{0, 1, 2}) { // the header and its entry no longer fit
		t.Errorf("tight span = %v", got)
	}
	if got, want := span(head, 3, 10), []int{-1, 3}; !reflect.DeepEqual(got, want) { // the header scrolls with it
		t.Errorf("span from the header's entry = %v, want %v", got, want)
	}
	if got := span(noHeads(4), 0, 10); len(got) != 4 { // no rules between entries
		t.Errorf("plain span = %v", got)
	}
}

func TestScrollTopKeepsTheCursorInView(t *testing.T) {
	head := noHeads(10)
	head[7] = true
	top := 0
	for pos := 0; pos < 10; pos++ {
		top = scrollTop(head, top, pos, 5)
		if !drawn(span(head, top, 5), pos) {
			t.Fatalf("cursor %d off screen with top %d", pos, top)
		}
	}
	if top = scrollTop(head, top, 0, 5); top != 0 {
		t.Errorf("scrolling back to the first entry left top at %d", top)
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/tmux"
)

func TestTitlePillFollowsTheMode(t *testing.T) {
	trueColor(t)
	const (
		yellow   = "249;226;175"
		blue     = "137;179;250"
		red      = "243;139;168"
		green    = "166;227;161"
		lavender = "179;190;254"
	)
	cases := []struct {
		name  string
		focus focus
		mode  tmux.Mode
		label string
		rgb   string
	}{
		{"idle", focusTerm, tmux.Mode{}, "VANTAGE", green},
		{"prefix", focusTerm, tmux.Mode{Prefix: true}, "PREFIX", yellow},
		{"copy", focusTerm, tmux.Mode{Copy: true}, "COPY", blue},
		{"sync", focusTerm, tmux.Mode{Sync: true}, "SYNC", red},
		{"prefix beats copy and sync", focusTerm, tmux.Mode{Prefix: true, Copy: true, Sync: true}, "PREFIX", yellow},
		{"copy beats sync", focusTerm, tmux.Mode{Copy: true, Sync: true}, "COPY", blue},
		{"sidebar", focusSidebar, tmux.Mode{}, "SIDEBAR", lavender},
		{"sidebar beats the tmux modes", focusSidebar, tmux.Mode{Prefix: true, Sync: true}, "SIDEBAR", lavender},
	}
	for _, c := range cases {
		m := model{focus: c.focus, mode: c.mode}
		p := m.pill()
		if got := strings.TrimSpace(ansi.Strip(p)); got != c.label {
			t.Errorf("%s: label %q, want %q", c.name, got, c.label)
		}
		if !strings.Contains(p, c.rgb) {
			t.Errorf("%s: wrong color in %q", c.name, p)
		}
	}
}

func TestSidebarTitleKeepsItsWidthInBothViews(t *testing.T) {
	m := model{w: 100, h: 24, focus: focusTerm, mode: tmux.Mode{Prefix: true}}
	for _, v := range []sidebarView{viewSessions, viewHosts} {
		m.view = v
		title := m.sidebar()[0]
		if w := ansi.StringWidth(title); w != sidebarW {
			t.Errorf("view %v: title width %d, want %d", v, w, sidebarW)
		}
	}
	m.view = viewHosts
	side := m.sidebar()
	if got := ansi.Strip(side[0]); !strings.Contains(got, "PREFIX") {
		t.Errorf("hosts title lost the mode: %q", got)
	}
	if got := ansi.Strip(side[1]); !strings.Contains(got, "H hosts") {
		t.Errorf("hosts tab strip lost its tabs: %q", got)
	}
}

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestHelpOpensAndCloses(t *testing.T) {
	for _, closer := range []string{"esc", "?", "q"} {
		m := treeModel()
		m.w, m.h = 100, 30
		m, _ = press(m, "?")
		if !m.help || !strings.Contains(popupText(m), "keys") {
			t.Fatalf("? must open the help pop-up: %q", popupText(m))
		}
		m, cmd := press(m, "j", "x") // other keys are ignored
		if !m.help || cmd != nil {
			t.Fatalf("help must swallow other keys")
		}
		if m, _ = press(m, closer); m.help {
			t.Errorf("%s must close help", closer)
		}
	}
}

func TestHelpQDoesNotQuit(t *testing.T) {
	m := treeModel()
	m, _ = press(m, "?")
	if _, cmd := press(m, "q"); cmd != nil {
		t.Error("q in help must only close it")
	}
}

func TestHelpFollowsView(t *testing.T) {
	m := treeModel()
	m.w, m.h = 100, 30
	m, _ = press(m, "?")
	if box := popupText(m); !strings.Contains(box, "new") || strings.Contains(box, "ssh split") {
		t.Errorf("workspaces help = %q", box)
	}
	m.help, m.view = false, viewHosts
	m, _ = press(m, "?")
	if box := popupText(m); !strings.Contains(box, "ssh split beside") || strings.Contains(box, "rename") {
		t.Errorf("hosts help = %q", box)
	}
}

func TestHelpColumnsByWidth(t *testing.T) {
	m := treeModel()
	m.help = true
	m.h = 40
	m.w = 100
	wide := m.helpBox()
	m.w = 40
	narrow := m.helpBox()
	if len(wide) >= len(narrow) {
		t.Errorf("two columns must be shorter: %d vs %d", len(wide), len(narrow))
	}
	for _, l := range narrow {
		if w := ansi.StringWidth(l); w > m.w-2 {
			t.Errorf("narrow line is %d cells wide in a %d window: %q", w, m.w, ansi.Strip(l))
		}
	}
}

func TestHelpClipsToHeight(t *testing.T) {
	m := treeModel()
	m.help, m.w, m.h = true, 40, 12
	if n := len(m.helpBox()); n > m.h {
		t.Errorf("pop-up is %d lines in a %d line window", n, m.h)
	}
}

func TestFooterOnlyHintsHelp(t *testing.T) {
	m := treeModel()
	m.w, m.h = 100, 30
	if side := ansi.Strip(strings.Join(m.sidebar(), "\n")); !strings.Contains(side, "? keys") || strings.Contains(side, "alt+s q") {
		t.Errorf("sidebar = %q", side)
	}
}

func TestHelpOpensFromTmuxFocus(t *testing.T) {
	for _, k := range []string{"alt+/", "alt+?"} {
		m := treeModel()
		m.focus = focusTerm
		next, _ := m.Update(altKey(k))
		if !next.(model).help {
			t.Errorf("%s must open help while tmux has focus", k)
		}
	}
}

func altKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s[len("alt+"):]), Alt: true}
}

func TestSidebarHeadingsNameTheView(t *testing.T) {
	m := treeModel()
	m.w, m.h = 100, 30
	if top := ansi.Strip(m.sidebar()[1]); !strings.Contains(top, "workspaces") {
		t.Errorf("workspaces heading = %q", top)
	}
	m.view = viewHosts
	if top := ansi.Strip(m.sidebar()[1]); !strings.Contains(top, "ssh hosts") {
		t.Errorf("hosts heading = %q", top)
	}
}

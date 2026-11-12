package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyBytes(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyMsg
		want string
		ok   bool
	}{
		{"rune", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("é")}, "é", true},
		{"alt rune", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h"), Alt: true}, "\x1bh", true},
		{"ctrl-a", tea.KeyMsg{Type: tea.KeyCtrlA}, "\x01", true},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, "\r", true},
		{"alt up", tea.KeyMsg{Type: tea.KeyUp, Alt: true}, "\x1b\x1b[A", true},
		{"esc", tea.KeyMsg{Type: tea.KeyEscape}, "\x1b", true},
		{"unknown", tea.KeyMsg{Type: tea.KeyType(-9999)}, "", false},
	}
	for _, c := range cases {
		b, ok := keyBytes(c.key)
		if ok != c.ok || string(b) != c.want {
			t.Errorf("%s: got %q,%v want %q,%v", c.name, b, ok, c.want, c.ok)
		}
	}
}

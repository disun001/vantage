package pick

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keys(m tea.Model, in ...string) menu {
	for _, s := range in {
		var k tea.KeyMsg
		switch s {
		case "enter":
			k = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			k = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			k = tea.KeyMsg{Type: tea.KeyDown}
		case "backspace":
			k = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			k = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
		}
		m, _ = m.Update(k)
	}
	return m.(menu)
}

func TestMenu(t *testing.T) {
	hosts := []string{"web-prod", "db-prod", "web-staging"}
	tests := []struct {
		name      string
		keys      []string
		alias     string
		cancelled bool
		done      bool // Enter or cancel was reached
	}{
		{"enter picks local", []string{"enter"}, "", false, true},
		{"j then enter picks first host", []string{"j", "enter"}, "web-prod", false, true},
		{"down arrow", []string{"down", "down", "enter"}, "db-prod", false, true},
		{"k wraps to last", []string{"k", "enter"}, "web-staging", false, true},
		{"G jumps to last", []string{"G", "enter"}, "web-staging", false, true},
		{"g jumps back", []string{"G", "g", "enter"}, "", false, true},
		{"filter picks best", []string{"/", "d", "b", "enter"}, "db-prod", false, true},
		{"filter is fuzzy", []string{"/", "w", "s", "enter"}, "web-staging", false, true},
		{"typing in normal mode does not filter", []string{"d", "b", "enter"}, "", false, true},
		{"esc leaves filter then clears it then cancels", []string{"/", "d", "esc", "esc", "esc"}, "", true, true},
		{"esc in filter keeps the filter", []string{"/", "d", "b", "esc", "enter"}, "db-prod", false, true},
		{"q cancels", []string{"q"}, "", true, true},
		{"no match: enter does nothing", []string{"/", "z", "z", "enter"}, "", false, false},
		{"backspace widens", []string{"/", "z", "backspace", "enter"}, "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := keys(newMenu(hosts), tt.keys...)
			if m.cancelled != tt.cancelled {
				t.Fatalf("cancelled = %v, want %v", m.cancelled, tt.cancelled)
			}
			if tt.done && m.choice.Alias != tt.alias {
				t.Fatalf("alias = %q, want %q", m.choice.Alias, tt.alias)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	items := []string{"local shell", "web-prod", "db-prod", "web-staging"}
	tests := []struct {
		pattern string
		want    []string
	}{
		{"", items},
		{"prod", []string{"db-prod", "web-prod"}},
		{"WS", []string{"web-staging"}},
		{"zzz", nil},
		{"loc", []string{"local shell"}},
	}
	for _, tt := range tests {
		got := filter(tt.pattern, items)
		if len(got) != len(tt.want) || (len(got) > 0 && !reflect.DeepEqual(got, tt.want)) {
			t.Errorf("filter(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

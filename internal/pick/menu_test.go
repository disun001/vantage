package pick

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/tmux"
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
	hosts := []Host{{Alias: "web-prod"}, {Alias: "db-prod", HostName: "192.0.2.20"}, {Alias: "web-staging"}}
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
			m := keys(newMenu(Context{Dir: "v"}, hosts), tt.keys...)
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
		want    []int
	}{
		{"", []int{0, 1, 2, 3}},
		{"prod", []int{2, 1}}, // shorter match first
		{"WS", []int{3}},
		{"zzz", nil},
		{"loc", []int{0}},
	}
	for _, tt := range tests {
		got := filter(tt.pattern, items)
		if len(got) != len(tt.want) || (len(got) > 0 && !reflect.DeepEqual(got, tt.want)) {
			t.Errorf("filter(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

func TestView(t *testing.T) {
	hosts := []Host{
		{
			Alias: "db-prod", HostName: "192.0.2.20", User: "admin", Port: "2222", ProxyJump: "bastion",
			Open: []tmux.PaneRef{{Session: "infra", Window: 2, WindowName: "logs"}},
		},
		{Alias: "nas"},
	}
	m := keys(newMenu(Context{Dir: "h", CWD: "/work"}, hosts), "j")
	v := m.View()
	for _, want := range []string{"db-prod", "192.0.2.20", "admin", "2222", "bastion", "ssh db-prod", "connected in 1 pane", "infra:2", "logs", "●"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
	m = keys(m, "j")
	if v := m.View(); !strings.Contains(v, "not connected") {
		t.Errorf("host without panes should say not connected:\n%s", v)
	}
	m = keys(newMenu(Context{Dir: "h", CWD: "/work"}, hosts))
	if v := m.View(); !strings.Contains(v, "/work") || !strings.Contains(v, "side by side") {
		t.Errorf("local detail missing cwd or direction:\n%s", v)
	}
	// Many open panes are summarised, and no match does not panic.
	var open []tmux.PaneRef
	for i := 0; i < 7; i++ {
		open = append(open, tmux.PaneRef{Session: "s", Window: i})
	}
	m = keys(newMenu(Context{Dir: "v"}, []Host{{Alias: "x", Open: open}}), "j")
	if v := m.View(); !strings.Contains(v, "+3 more") {
		t.Errorf("want +3 more:\n%s", v)
	}
	_ = keys(newMenu(Context{Dir: "v"}, hosts), "/", "z", "z", "z").View()
}

func TestMenuChatRow(t *testing.T) {
	ctx := Context{Dir: "h", CWD: "/work", ChatExe: "/opt/bin/vantage"}
	m := keys(newMenu(ctx, []Host{{Alias: "web"}}), "j", "enter")
	if !m.choice.Chat || m.choice.Exe != ctx.ChatExe || m.choice.Alias != "" {
		t.Fatalf("choice = %+v, want chat with exe", m.choice)
	}
	if !strings.Contains(m.View(), "Claude chat (local)") {
		t.Error("view does not list the chat row")
	}
	if m := keys(newMenu(ctx, nil), "/", "a", "i", "enter"); !m.choice.Chat {
		t.Error("filter 'ai' should reach the chat row")
	}
	if len(newMenu(Context{}, nil).items) != 1 {
		t.Error("chat row must be hidden without ChatExe")
	}
}

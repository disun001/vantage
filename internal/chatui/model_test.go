package chatui

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/chat"
	"github.com/disun001/vantage/internal/store"
)

type fakeDriver struct {
	mu         sync.Mutex
	sent       []string
	interrupts int
}

func (f *fakeDriver) Send(_ context.Context, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeDriver) Interrupt(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.interrupts++
	return nil
}

func (f *fakeDriver) Close() error { return nil }

func ev(kind, text string) store.Event { return store.Event{Kind: kind, Text: text} }

func sized(t *testing.T, d Driver, hist ...store.Event) model {
	t.Helper()
	m := newModel(context.Background(), d, "chat abc", hist, true)
	return m.resize(60, 20)
}

func typeText(m model, s string) model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return next.(model)
}

func press(m model, k tea.KeyType) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: k})
	return next.(model), cmd
}

func TestRenderTranscript(t *testing.T) {
	out := render([]store.Event{
		ev(chat.KindSession, "model=m auth=subscription"),
		ev(chat.KindUserMsg, "hello"),
		ev(chat.KindToolUse, "Read(main.go)"),
		ev(chat.KindToolResult, "SECRET RESULT"),
		ev(chat.KindAssistantMsg, "hi there"),
		ev(chat.KindTurnEnd, ""),
		ev(chat.KindError, "boom"),
	}, 40, newMarkdown(40, true), false)
	out = ansi.Strip(out) // glamour styles word by word
	for _, want := range []string{"hello", "Read(main.go)", "hi there", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("transcript missing %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"SECRET RESULT", "model=m"} {
		if strings.Contains(out, bad) {
			t.Errorf("transcript should not show %q", bad)
		}
	}
}

func TestBusy(t *testing.T) {
	tests := []struct {
		name string
		ev   []store.Event
		want bool
	}{
		{"empty", nil, false},
		{"sent", []store.Event{ev(chat.KindUserMsg, "x")}, true},
		{"mid turn", []store.Event{ev(chat.KindUserMsg, "x"), ev(chat.KindToolUse, "t")}, true},
		{"ended", []store.Event{ev(chat.KindUserMsg, "x"), ev(chat.KindTurnEnd, "")}, false},
		{"errored", []store.Event{ev(chat.KindUserMsg, "x"), ev(chat.KindError, "e")}, false},
		{"second turn", []store.Event{ev(chat.KindUserMsg, "x"), ev(chat.KindTurnEnd, ""), ev(chat.KindUserMsg, "y")}, true},
	}
	for _, tt := range tests {
		if got := busy(tt.ev); got != tt.want {
			t.Errorf("%s: busy = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEnterSendsAndClearsInput(t *testing.T) {
	d := &fakeDriver{}
	m := typeText(sized(t, d), "  hello  ")
	m, cmd := press(m, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("enter returned no command")
	}
	cmd()
	if len(d.sent) != 1 || d.sent[0] != "hello" {
		t.Fatalf("sent = %q, want [hello]", d.sent)
	}
	if m.input.Value() != "" {
		t.Errorf("input not cleared: %q", m.input.Value())
	}
}

func TestEnterIgnoredWhenEmptyOrBusy(t *testing.T) {
	d := &fakeDriver{}
	if _, cmd := press(sized(t, d), tea.KeyEnter); cmd != nil {
		t.Error("empty input should not send")
	}
	m := typeText(sized(t, d, ev(chat.KindUserMsg, "earlier")), "next")
	if _, cmd := press(m, tea.KeyEnter); cmd != nil {
		t.Error("busy chat should not send")
	}
}

func TestEventAppendsAndEndsBusy(t *testing.T) {
	m := sized(t, &fakeDriver{})
	next, _ := m.Update(eventMsg(ev(chat.KindUserMsg, "q")))
	m = next.(model)
	if !busy(m.events) || !strings.Contains(m.View(), "working") {
		t.Fatalf("want busy status, got:\n%s", m.View())
	}
	next, _ = m.Update(eventMsg(ev(chat.KindAssistantMsg, "answer")))
	next, _ = next.(model).Update(eventMsg(ev(chat.KindTurnEnd, "")))
	m = next.(model)
	if busy(m.events) || !strings.Contains(m.View(), "answer") {
		t.Fatalf("want idle with answer, got:\n%s", m.View())
	}
}

func TestCtrlCInterruptsThenQuits(t *testing.T) {
	d := &fakeDriver{}
	m := sized(t, d, ev(chat.KindUserMsg, "q"))
	_, cmd := press(m, tea.KeyCtrlC)
	cmd()
	if d.interrupts != 1 {
		t.Fatalf("interrupts = %d, want 1", d.interrupts)
	}
	idle := sized(t, d)
	_, cmd = press(idle, tea.KeyCtrlC)
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c on an idle chat should quit")
	}
}

func TestSendErrorShownInStatus(t *testing.T) {
	m := sized(t, &fakeDriver{})
	next, _ := m.Update(sentMsg{err: context.DeadlineExceeded})
	if !strings.Contains(next.(model).View(), "deadline") {
		t.Error("send error not shown")
	}
}

func TestToolResultsToggle(t *testing.T) {
	evs := []store.Event{ev(chat.KindToolUse, "Read(a)"), ev(chat.KindToolResult, "line one\nline two")}
	md := newMarkdown(40, true)
	if strings.Contains(render(evs, 40, md, false), "line one") {
		t.Error("collapsed transcript shows tool result")
	}
	if !strings.Contains(render(evs, 40, md, true), "line two") {
		t.Error("expanded transcript misses tool result")
	}
	m := sized(t, &fakeDriver{}, evs...)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	if !next.(model).expand || !strings.Contains(next.(model).View(), "line one") {
		t.Error("ctrl+o did not expand")
	}
}

func TestMarkdownRenders(t *testing.T) {
	out := newMarkdown(40, true).render("# Title\n\n- item **bold**", 40)
	if !strings.Contains(out, "Title") || !strings.Contains(out, "item") || strings.Contains(out, "**") {
		t.Errorf("markdown not rendered: %q", out)
	}
	if got := (markdown{}).render("plain text", 40); !strings.Contains(got, "plain text") {
		t.Errorf("fallback = %q", got)
	}
}

func TestUsage(t *testing.T) {
	raw := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	evs := []store.Event{
		{Kind: chat.KindTurnEnd, Raw: raw(`{"usage":{"input_tokens":4,"cache_creation_input_tokens":100,"cache_read_input_tokens":900,"output_tokens":50}}`)},
		{Kind: chat.KindTurnEnd, Raw: raw(`{"usage":{"input_tokens":6,"cache_read_input_tokens":1994,"output_tokens":25}}`)},
		{Kind: chat.KindTurnEnd, Raw: raw(`not json`)},
	}
	c, o := usage(evs)
	if c != 2000 || o != 75 {
		t.Errorf("usage = %d, %d; want 2000, 75", c, o)
	}
	if kilo(151) != "151" || kilo(58000) != "58.0k" {
		t.Error("kilo format")
	}
}

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/limits"
)

func TestCountdown(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-time.Minute, "0m"},
		{45 * time.Minute, "45m"},
		{2*time.Hour + 10*time.Minute, "2h10m"},
		{2*time.Hour + 5*time.Minute, "2h05m"},
		{4*24*time.Hour + 2*time.Hour + 20*time.Minute, "4d2h"},
	}
	for _, tc := range tests {
		if got := countdown(tc.d); got != tc.want {
			t.Errorf("countdown(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestLimitLine(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tests := []struct {
		name string
		w    limits.Window
		want string
	}{
		{"normal", limits.Window{Used: 0.24, ResetsAt: now.Add(2*time.Hour + 10*time.Minute), Known: true}, "5h ▓▓░░░░░░  24% ↻2h10m"},
		{"over", limits.Window{Used: 1.7, ResetsAt: now.Add(time.Hour), Known: true}, "5h ▓▓▓▓▓▓▓▓ 100% ↻1h00m"},
		{"reset passed", limits.Window{Used: 0.9, ResetsAt: now.Add(-time.Minute), Known: true}, "5h ░░░░░░░░  --"},
		{"unknown", limits.Window{}, "5h ░░░░░░░░  --"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(limitLine("5h", tc.w, now, false)); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func limitModel() model {
	cfg := Config{Limits: func() (limits.Snapshot, error) { return limits.Snapshot{}, nil }}
	m := newModel(cfg, &hub{})
	m.w, m.h = 100, 30
	return m
}

func goodSnap() limits.Snapshot {
	w := limits.Window{Used: 0.5, ResetsAt: time.Now().Add(time.Hour), Known: true}
	return limits.Snapshot{FiveHour: w, SevenDay: w}
}

func TestLimitRowsVisibility(t *testing.T) {
	m := limitModel()
	if rows := m.limitRows(); rows != nil {
		t.Fatalf("hidden before a probe works, got %v", rows)
	}
	m.limits = m.limits.onLimits(limitsMsg{err: errors.New("no claude")})
	if rows := m.limitRows(); rows != nil {
		t.Fatalf("still hidden after a failed first probe, got %v", rows)
	}
	m.limits = m.limits.onLimits(limitsMsg{snap: goodSnap()})
	if rows := m.limitRows(); len(rows) != 2 {
		t.Fatalf("want 2 rows after a good probe, got %v", rows)
	}
	short := m
	short.h = limitMinH - 1
	if rows := short.limitRows(); rows != nil {
		t.Errorf("dropped on a short terminal, got %v", rows)
	}
	off := m
	off.cfg.Limits = nil
	if rows := off.limitRows(); rows != nil {
		t.Errorf("hidden when off, got %v", rows)
	}
}

func TestLimitStateDimsAfterMisses(t *testing.T) {
	s := limitState{}.onLimits(limitsMsg{snap: goodSnap()})
	s = s.onLimits(limitsMsg{err: errors.New("x")})
	if s.misses != 1 || !s.ok {
		t.Fatalf("one miss keeps the data: %+v", s)
	}
	s = s.onLimits(limitsMsg{snap: goodSnap()})
	if s.misses != 0 {
		t.Errorf("a good probe clears the misses: %+v", s)
	}
}

func TestFootSitsAboveHint(t *testing.T) {
	m := limitModel()
	m.limits = m.limits.onLimits(limitsMsg{snap: goodSnap()})
	for name, lines := range map[string][]string{
		"sessions": m.sidebar(),
		"hosts":    m.hostsSidebar(),
		"projects": m.projectsSidebar(),
	} {
		n := len(lines)
		if got := ansi.Strip(lines[n-1]); !strings.HasPrefix(got, "? keys") {
			t.Errorf("%s: last line %q", name, got)
		}
		if got := ansi.Strip(lines[n-2]); !strings.HasPrefix(got, "wk") {
			t.Errorf("%s: line above hint %q", name, got)
		}
		if got := ansi.Strip(lines[n-3]); !strings.HasPrefix(got, "5h") {
			t.Errorf("%s: two above hint %q", name, got)
		}
	}
}

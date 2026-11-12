package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNoticeWrapsInFull(t *testing.T) {
	long := "/var/folders/tz/sjmhkqz13/T/tmp.RANYzCd3og/workspaces/saved1.toml: unknown keys: windows, windows.name"
	m := model{w: 100, h: 24, notice: long, rows: []row{{Name: "a", Live: true}}}
	lines := m.noticeLines()
	if len(lines) < 2 || len(lines) > maxNoticeLines {
		t.Fatalf("got %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) > sidebarW {
			t.Errorf("line %q wider than the sidebar", l)
		}
	}
	if got := strings.Join(lines, ""); !strings.Contains(strings.ReplaceAll(got, " ", ""), "unknownkeys:windows,windows.name") {
		t.Errorf("text lost: %q", got)
	}
	if v := m.View(); strings.Count(v, "\n")+1 != m.h {
		t.Errorf("view has %d lines, want %d", strings.Count(v, "\n")+1, m.h)
	}

	m.notice = strings.Repeat("word ", 60)
	if l := m.noticeLines(); len(l) != maxNoticeLines || !strings.HasSuffix(l[len(l)-1], "…") {
		t.Errorf("long notice not capped with ellipsis: %q", l)
	}
	m.h = 6
	if m.noticeLines() != nil {
		t.Error("tiny window should drop the notice, not the rows")
	}
}

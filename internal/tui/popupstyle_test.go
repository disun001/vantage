package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const (
	mantleBG = "48;2;24;24;36" // #181825, the panel
	redBG    = "48;2;243;139;168"
	mauveBG  = "48;2;203;166;247"
)

func TestFramedPanelKeepsItsShapeAndColors(t *testing.T) {
	trueColor(t)
	rows := []string{popTitle("new workspace"), "", noticeStyle.Render("careful"), curStyle.Render("name ") + "abc" + cursorStyle.Render(" ")}
	box := framed(rows, 20)
	if len(box) != len(rows)+2 {
		t.Fatalf("height %d, want %d", len(box), len(rows)+2)
	}
	for i, l := range box {
		if w := ansi.StringWidth(l); w != 24 {
			t.Errorf("line %d: width %d, want 24: %q", i, w, ansi.Strip(l))
		}
	}
	plain := ansi.Strip(strings.Join(box, "\n"))
	if !strings.HasPrefix(plain, "╭") || !strings.HasSuffix(plain, "╯") {
		t.Errorf("border: %q", plain)
	}
	// After every reset inside a row the panel background starts again, so
	// no cell falls back to the terminal's own.
	for i, l := range box[1 : len(box)-1] {
		body := l[strings.Index(l, "│")+len("│"):]
		body = body[:strings.LastIndex(body, "│")]
		for _, part := range strings.Split(body, "\x1b[0m")[1:] {
			if part == "" {
				continue
			}
			if !strings.HasPrefix(part, "\x1b[") || !strings.Contains(part[:strings.Index(part, "m")], mantleBG) {
				t.Errorf("row %d loses the panel background after a reset: %q", i, part)
			}
		}
	}
}

func TestDestructivePopupsHaveARedTitle(t *testing.T) {
	trueColor(t)
	if !strings.Contains(dangerTitle("delete host"), redBG) {
		t.Error("danger title is not red")
	}
	if !strings.Contains(popTitle("add host"), mauveBG) {
		t.Error("title is not mauve")
	}
	m := model{w: 100, h: 24, prompt: &prompt{kind: promptDelete, target: "api"}}
	if !strings.Contains(strings.Join(m.deleteBox(), "\n"), redBG) {
		t.Error("delete workspace pop-up has no red title")
	}
	m.prompt = &prompt{kind: promptNew}
	if strings.Contains(strings.Join(m.promptBox(), "\n"), redBG) {
		t.Error("new workspace pop-up is red")
	}
}

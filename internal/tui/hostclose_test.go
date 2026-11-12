package tui

import (
	"io"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func askToClose(t *testing.T, u *fakeUse) model {
	t.Helper()
	m, _ := usedHost(t, u)
	m.w, m.h = 80, 24
	m, cmd := press(m, "x")
	if cmd == nil {
		t.Fatal("x did nothing")
	}
	return settle(m, cmd)
}

func TestCloseAllAsksThenClosesTheHostsPanes(t *testing.T) {
	u := panesAt(paneA, paneB)
	m := askToClose(t, u)
	if m.hostShut == nil {
		t.Fatalf("no question: notice %q", m.notice)
	}
	box := popupText(m)
	for _, want := range []string{"close panes of web", "proj:1 build", "ops:2 logs", "2 open pane(s) will be closed", "y close"} {
		if !strings.Contains(box, want) {
			t.Errorf("pop-up lacks %q:\n%s", want, box)
		}
	}
	lines := strings.Split(m.View(), "\n")
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("line %d is %d cells wide", i, w)
		}
	}
	if len(u.closed) != 0 {
		t.Fatalf("closed before the answer: %v", u.closed)
	}
	m, cmd := press(m, "y")
	if m.hostShut != nil || cmd == nil {
		t.Fatal("y must close the question and run")
	}
	m = settleAll(m, cmd)
	if !reflect.DeepEqual(u.closed, []string{"%3", "%7"}) || m.notice != "closed 2 pane(s) of web" {
		t.Errorf("closed %v, notice %q", u.closed, m.notice)
	}
}

func TestCloseAllCancelClosesNothing(t *testing.T) {
	for _, k := range []string{"n", "esc", "enter", "x"} {
		u := panesAt(paneA)
		m := askToClose(t, u)
		m, cmd := press(m, k)
		if m.hostShut != nil || cmd != nil || len(u.closed) != 0 {
			t.Errorf("%s: open=%v cmd=%v closed=%v", k, m.hostShut != nil, cmd != nil, u.closed)
		}
	}
}

func TestCloseAllWithNoPanesSaysSo(t *testing.T) {
	m := askToClose(t, panesAt())
	if m.hostShut != nil || m.notice != "no open panes for web" {
		t.Errorf("notice %q", m.notice)
	}
}

func TestCloseAllReportsFailures(t *testing.T) {
	m := askToClose(t, &fakeUse{findErr: io.ErrUnexpectedEOF})
	if !strings.Contains(m.notice, "could not look for open panes") {
		t.Errorf("lookup: %q", m.notice)
	}
	u := panesAt(paneA)
	u.closeErr = io.ErrClosedPipe
	m = askToClose(t, u)
	m, cmd := press(m, "y")
	m = settleAll(m, cmd)
	if !strings.Contains(m.notice, "closing panes of web failed") {
		t.Errorf("close: %q", m.notice)
	}
}

func TestCloseAllDoesNothingWithoutAFinder(t *testing.T) {
	m := openHosts(t, twoHosts())
	if m, cmd := press(m, "x"); cmd != nil || m.hostShut != nil {
		t.Error("x needs a usage finder")
	}
}

func TestMouseIgnoredWhileCloseAllOpen(t *testing.T) {
	m := askToClose(t, panesAt(paneA))
	if next, _ := m.Update(tea.MouseMsg{X: 3, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}); next.(model).hostShut == nil {
		t.Error("the mouse reached past the question")
	}
}

package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestPaneTitlePromptStartsWithCurrentName(t *testing.T) {
	m := treeModel()
	m = m.onPaneTitle(paneTitleMsg{pane: "%3", title: "api"})
	if m.prompt == nil || m.prompt.text != "api" || m.prompt.target != "%3" {
		t.Fatalf("prompt = %+v", m.prompt)
	}
	if box := popupText(m); !strings.Contains(box, "rename pane") || !strings.Contains(box, "api") {
		t.Errorf("pop-up = %q", box)
	}
}

func TestPaneTitleLookupFailureShowsNotice(t *testing.T) {
	m := treeModel().onPaneTitle(paneTitleMsg{err: errors.New("tmux client not ready, try again")})
	if m.prompt != nil || m.notice == "" {
		t.Errorf("prompt = %+v, notice = %q", m.prompt, m.notice)
	}
}

func TestPaneTitlePromptSetsClearsAndSkips(t *testing.T) {
	c := testClient(t)
	pane, err := c.NewSession("p", "w", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := treeModel()
	m.cfg.Client = c
	open := func(cur string) model {
		if cur != "" {
			if err := c.SetPaneTitle(pane, cur); err != nil {
				t.Fatal(err)
			}
		}
		return m.onPaneTitle(paneTitleMsg{pane: pane, title: cur})
	}

	// Typing after the current name appends; enter saves it.
	got, cmd := press(open("api"), "-", "2", "enter")
	if got.prompt != nil || cmd == nil {
		t.Fatalf("enter must close the prompt and run: %+v", got.prompt)
	}
	if msg := cmd().(manageMsg); msg.err != nil {
		t.Fatal(msg.err)
	}
	if title, _ := c.PaneTitle(pane); title != "api-2" {
		t.Errorf("title = %q", title)
	}

	// An empty name is allowed here and clears the custom name.
	got, cmd = press(open("api-2"), "ctrl+u", "enter")
	if cmd == nil {
		t.Fatal("clearing must run")
	}
	if msg := cmd().(manageMsg); msg.err != nil {
		t.Fatal(msg.err)
	}
	if title, _ := c.PaneTitle(pane); title != "" {
		t.Errorf("title after clear = %q", title)
	}

	// Nothing changed: nothing runs.
	if _, cmd = press(open("same"), "enter"); cmd != nil {
		t.Error("an unchanged name must do nothing")
	}
	// Esc cancels quietly.
	if got, cmd = press(open("keep"), "x", "esc"); got.prompt != nil || cmd != nil {
		t.Error("esc must cancel")
	}
	if title, _ := c.PaneTitle(pane); title != "keep" {
		t.Errorf("title after esc = %q", title)
	}

	// A name vantage refuses is reported and not applied.
	_, cmd = press(open("keep"), "#", "enter")
	msg := cmd().(manageMsg)
	if msg.err == nil {
		t.Fatal("want an error for '#'")
	}
	if title, _ := c.PaneTitle(pane); title != "keep" {
		t.Errorf("rejected name was applied: %q", title)
	}
	next, _ := m.onManage(msg)
	if !strings.Contains(next.(model).notice, "#") {
		t.Errorf("notice = %q", next.(model).notice)
	}
}

func TestAltRNeedsATerminal(t *testing.T) {
	m, cmd := press(treeModel(), "alt+r")
	if m.prompt != nil || cmd != nil {
		t.Error("alt+r without an embedded client must do nothing")
	}
}

func TestPaneTitleMsgReachesThePrompt(t *testing.T) {
	next, _ := treeModel().Update(paneTitleMsg{pane: "%1", title: "x"})
	if got := next.(model); got.prompt == nil || got.prompt.kind != promptRenamePane {
		t.Fatalf("prompt = %+v", got.prompt)
	}
}

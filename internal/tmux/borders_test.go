package tmux

import (
	"strings"
	"testing"
)

var testColors = PaneBorderColors{Inactive: "#45475a", Active: "#b4befe", Label: "#a6adc8"}

func TestApplyPaneBorders(t *testing.T) {
	c := testClient(t)
	if _, err := c.NewSession("b", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// The user already customised the active style: it must survive.
	if _, err := c.run("set-option", "-gw", "pane-active-border-style", "fg=red"); err != nil {
		t.Fatal(err)
	}
	c.ApplyPaneBorders(testColors)
	want := map[string]string{
		"pane-border-style":        "fg=#45475a",
		"pane-active-border-style": "fg=red",
		"pane-border-status":       "top",
	}
	for opt, v := range want {
		got, err := c.run("show-options", "-gwv", opt)
		if err != nil || got != v {
			t.Errorf("%s = %q (err %v), want %q", opt, got, err, v)
		}
	}
}

func TestApplyPaneBordersKeepsUserLabel(t *testing.T) {
	c := testClient(t)
	if _, err := c.NewSession("b", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.run("set-option", "-gw", "pane-border-format", " mine "); err != nil {
		t.Fatal(err)
	}
	if _, err := c.run("set-option", "-gw", "pane-border-status", "bottom"); err != nil {
		t.Fatal(err)
	}
	c.ApplyPaneBorders(testColors)
	for opt, v := range map[string]string{"pane-border-format": " mine ", "pane-border-status": "bottom"} {
		if got, _ := c.run("show-options", "-gwv", opt); got != v {
			t.Errorf("%s = %q, want the user's %q", opt, got, v)
		}
	}
}

func TestPaneTitle(t *testing.T) {
	c := testClient(t)
	pane, err := c.NewSession("t", "w", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.ApplyPaneBorders(testColors)
	if got, _ := c.PaneTitle(pane); got != "" {
		t.Fatalf("fresh pane title = %q, want empty", got)
	}
	if err := c.SetPaneTitle(pane, "api | server"); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.PaneTitle(pane); got != "api | server" {
		t.Errorf("title = %q", got)
	}
	// The border label shows the custom name, then falls back to the command.
	label := func() string {
		out, err := c.run("display-message", "-p", "-t", pane, "-F", "#{T:pane-border-format}")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := label(); !strings.Contains(got, "api | server") {
		t.Errorf("label %q lacks the custom name", got)
	}
	titles, err := c.PaneTitles("t")
	if err != nil || titles[pane] != "api | server" {
		t.Errorf("PaneTitles = %v, %v", titles, err)
	}
	if err := c.SetPaneTitle(pane, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.PaneTitle(pane); got != "" {
		t.Errorf("cleared title = %q", got)
	}
	if got := label(); strings.Contains(got, "api") {
		t.Errorf("label %q still shows the cleared name", got)
	}
	for _, bad := range []string{"x", "%x", "%1;kill-server"} {
		if err := c.SetPaneTitle(bad, "t"); err == nil {
			t.Errorf("SetPaneTitle(%q) accepted a bad pane id", bad)
		}
	}
}

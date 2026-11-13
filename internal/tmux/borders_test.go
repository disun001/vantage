package tmux

import "testing"

func TestApplyPaneBorders(t *testing.T) {
	c := testClient(t)
	if _, err := c.NewSession("b", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// The user already customised the active style: it must survive.
	if _, err := c.run("set-option", "-gw", "pane-active-border-style", "fg=red"); err != nil {
		t.Fatal(err)
	}
	c.ApplyPaneBorders("#45475a", "#b4befe")
	want := map[string]string{
		"pane-border-style":        "fg=#45475a",
		"pane-active-border-style": "fg=red",
	}
	for opt, v := range want {
		got, err := c.run("show-options", "-gwv", opt)
		if err != nil || got != v {
			t.Errorf("%s = %q (err %v), want %q", opt, got, err, v)
		}
	}
}

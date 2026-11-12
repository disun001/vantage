package workspace

import (
	"os"
	"strings"
	"testing"
)

func TestPathOfNeverEscapesDir(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "../x"} {
		if p, err := PathOf(bad); err == nil {
			t.Errorf("PathOf(%q) = %q, want an error", bad, p)
		}
	}
	// A name ending in .toml stays a name: Resolve would read it as a path.
	p, err := PathOf("x.toml")
	if err != nil || !strings.HasSuffix(p, "workspaces/x.toml.toml") {
		t.Errorf("PathOf(x.toml) = %q, %v", p, err)
	}
}

func TestRenameWithoutTopLevelNameUsesFilename(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	src, _ := PathOf("one")
	body := "[[window]]\nname = \"one\"\n[[window.pane]]\n"
	if err := os.MkdirAll(src[:strings.LastIndex(src, "/")], 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Rename("one", "two"); err != nil {
		t.Fatal(err)
	}
	dst, _ := PathOf("two")
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != body {
		t.Fatalf("window name must not be touched: %q, %v", got, err)
	}
	if ws, err := Load(dst); err != nil || ws.Name != "two" {
		t.Errorf("load: %+v, %v", ws, err)
	}
}

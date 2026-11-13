package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRootPrecedence(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	t.Setenv("VANTAGE_PROJECTS_DIR", "")

	if _, err := Root(); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("unset: want ErrNoRoot, got %v", err)
	}

	writeFile(t, filepath.Join(cfg, "config.toml"), `projects_dir = "/srv/from-config"`)
	if got, err := Root(); err != nil || got != "/srv/from-config" {
		t.Fatalf("config key: got %q, %v", got, err)
	}

	t.Setenv("VANTAGE_PROJECTS_DIR", "/srv/from-env")
	if got, err := Root(); err != nil || got != "/srv/from-env" {
		t.Fatalf("env wins: got %q, %v", got, err)
	}
}

func TestRootExpandsHomeAndRejectsRelative(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Setenv("VANTAGE_PROJECTS_DIR", "~/projects")
	if got, err := Root(); err != nil || got != filepath.Join(home, "projects") {
		t.Errorf("got %q, %v", got, err)
	}
	t.Setenv("VANTAGE_PROJECTS_DIR", "projects")
	if _, err := Root(); err == nil {
		t.Error("relative path must be rejected")
	}
}

func TestRootRejectsUnknownConfigKeys(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	t.Setenv("VANTAGE_PROJECTS_DIR", "")
	writeFile(t, filepath.Join(cfg, "config.toml"), `projcts_dir = "/x"`)
	if _, err := Root(); err == nil {
		t.Error("typo in config.toml must be an error")
	}
}

func TestLoadAndList(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	if names, err := List(); err != nil || len(names) != 0 {
		t.Fatalf("empty: %v, %v", names, err)
	}
	writeFile(t, filepath.Join(cfg, "projects", "b.toml"), "[[repo]]\nurl = \"https://example.invalid/x.git\"\n")
	writeFile(t, filepath.Join(cfg, "projects", "a.toml"), "[[repo]]\nurl = \"https://example.invalid/y.git\"\n")
	names, err := List()
	if err != nil || len(names) != 2 || names[0] != "a" {
		t.Fatalf("list = %v, %v", names, err)
	}
	p, err := Load("b")
	if err != nil || p.Name != "b" {
		t.Fatalf("load = %+v, %v", p, err)
	}
	if _, err := Load("missing"); err == nil {
		t.Error("missing project must error")
	}
	for _, bad := range []string{"../x", "a/b", "", ".."} {
		if _, err := Load(bad); err == nil {
			t.Errorf("Load(%q) must be rejected", bad)
		}
	}
}

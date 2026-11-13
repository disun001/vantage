package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFull(t *testing.T) {
	p := write(t, "api.toml", `
root = "~/Work/api"

[[window]]
name = "editor"
layout = "main-vertical"

  [[window.pane]]
  cwd = "."
  cmd = "nvim"
  autorun = true

  [[window.pane]]
  ssh = "prod-db"
  split = "v"
`)
	ws, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Name != "api" {
		t.Errorf("name defaults to file base name, got %q", ws.Name)
	}
	if len(ws.Windows) != 1 || len(ws.Windows[0].Panes) != 2 {
		t.Fatalf("unexpected shape: %+v", ws)
	}
	if got := ws.Windows[0].Panes[1]; got.SSH != "prod-db" || got.Split != "v" {
		t.Errorf("pane 2 = %+v", got)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	p := write(t, "x.toml", "[[window]]\n[[window.pane]]\ncmdd = \"typo\"\n")
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "unknown keys") {
		t.Fatalf("want unknown keys error, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	pane := func(p Pane) Workspace {
		return Workspace{Name: "ok", Windows: []Window{{Panes: []Pane{p}}}}
	}
	cases := []struct {
		name string
		ws   Workspace
		bad  bool
	}{
		{"valid", pane(Pane{}), false},
		{"bad name", Workspace{Name: "a b", Windows: []Window{{Panes: []Pane{{}}}}}, true},
		{"dotted name", Workspace{Name: "a.b", Windows: []Window{{Panes: []Pane{{}}}}}, true},
		{"no windows", Workspace{Name: "ok"}, true},
		{"no panes", Workspace{Name: "ok", Windows: []Window{{}}}, true},
		{"bad split", pane(Pane{Split: "x"}), true},
		{"ssh and cmd", pane(Pane{SSH: "h", Cmd: "ls"}), true},
		{"ssh option injection", pane(Pane{SSH: "-oProxyCommand=evil"}), true},
		{"ssh with space", pane(Pane{SSH: "a b"}), true},
		{"autorun without cmd", pane(Pane{Autorun: true}), true},
		{"user@host alias ok", pane(Pane{SSH: "me@prod-1.example.com"}), false},
	}
	for _, c := range cases {
		err := c.ws.Validate()
		if (err != nil) != c.bad {
			t.Errorf("%s: err=%v, want bad=%v", c.name, err, c.bad)
		}
	}
}

func TestResolveCWD(t *testing.T) {
	home, _ := os.UserHomeDir()
	w := Workspace{Root: "~/proj"}
	cases := []struct{ cwd, want string }{
		{"", filepath.Join(home, "proj")},
		{".", filepath.Join(home, "proj")},
		{"web", filepath.Join(home, "proj", "web")},
		{"/etc", "/etc"},
		{"~/other", filepath.Join(home, "other")},
	}
	for _, c := range cases {
		if got := w.ResolveCWD(Pane{CWD: c.cwd}); got != c.want {
			t.Errorf("cwd %q: got %q want %q", c.cwd, got, c.want)
		}
	}
	if got := (Workspace{}).ResolveCWD(Pane{}); got != "" {
		t.Errorf("no root and no cwd should defer to tmux, got %q", got)
	}
}

func TestDirPrecedence(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if d, _ := Dir(); d != "/x/vantage/workspaces" {
		t.Errorf("xdg: %q", d)
	}
	t.Setenv("VANTAGE_CONFIG_DIR", "/v")
	if d, _ := Dir(); d != "/v/workspaces" {
		t.Errorf("override: %q", d)
	}
}

func TestValidateProject(t *testing.T) {
	win := []Window{{Panes: []Pane{{}}}}
	cases := []struct {
		name string
		ws   Workspace
		want string
	}{
		{"none", Workspace{Name: "w", Windows: win}, ""},
		{"project", Workspace{Name: "w", Project: "api", Windows: win}, ""},
		{"bad project", Workspace{Name: "w", Project: "a/b", Windows: win}, "invalid project"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.ws.Validate()
			switch {
			case c.want == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Errorf("want %q, got %v", c.want, err)
			}
		})
	}
}

func TestSaveKeepsLinkOfReplacedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.toml")
	win := []Window{{Panes: []Pane{{}}}}
	if err := Save(path, Workspace{Name: "w", Project: "api", Windows: win}, false); err != nil {
		t.Fatal(err)
	}
	// A capture knows nothing of projects.
	if err := Save(path, Workspace{Name: "w", Windows: win}, true); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Project != "api" {
		t.Errorf("link lost: %+v, %v", got, err)
	}
}

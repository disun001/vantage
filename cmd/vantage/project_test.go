package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/tui"
)

func TestProjectCommand(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	src := t.TempDir()
	cmd := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"init", "-q", "-b", "main")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	cmd = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "init")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	cfg, root := t.TempDir(), t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	t.Setenv("VANTAGE_PROJECTS_DIR", root)
	if err := os.MkdirAll(filepath.Join(cfg, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	def := "[[repo]]\nurl = \"" + src + "\"\ndir = \"api\"\n"
	if err := os.WriteFile(filepath.Join(cfg, "projects", "demo.toml"), []byte(def), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	steps := []struct {
		args []string
		want string
	}{
		{[]string{"project", "list"}, "demo"},
		{[]string{"project", "status", "demo"}, "not cloned"},
		{[]string{"project", "sync", "demo"}, "cloned  api"},
		{[]string{"project", "status", "demo"}, "up to date"},
	}
	for _, s := range steps {
		out.Reset()
		if code := run(s.args, &out, &errOut); code != 0 || !strings.Contains(out.String(), s.want) {
			t.Fatalf("%v: code=%d want %q in:\n%s\n%s", s.args, code, s.want, out.String(), errOut.String())
		}
	}
}

func TestProjectCommandErrors(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	t.Setenv("VANTAGE_PROJECTS_DIR", "")
	for _, args := range [][]string{
		{"project"},
		{"project", "bogus"},
		{"project", "sync"},
		{"project", "sync", "../x"},
		{"project", "status", "missing"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code == 0 {
			t.Errorf("%v: want non-zero exit", args)
		}
	}
}

func TestParseTaskArgs(t *testing.T) {
	got, err := parseTaskArgs([]string{"p", "--repo", "api:dev", "t", "--repo=web", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.pos) != 2 || !got.force || len(got.repos) != 2 ||
		got.repos[0].Base != "dev" || got.repos[1].Dir != "web" || got.repos[1].Base != "" {
		t.Errorf("parsed %+v", got)
	}
	for _, bad := range [][]string{{"--repo"}, {"--repo", ":x"}, {"--repo", "a:"}, {"--nope"}} {
		if _, err := parseTaskArgs(bad); err == nil {
			t.Errorf("%v should fail", bad)
		}
	}
}

func TestProjectStore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	src := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	}
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	t.Setenv("VANTAGE_PROJECTS_DIR", t.TempDir())

	s := projectStore{}
	if err := s.Create(tui.ProjectDef{Name: "demo", Repos: []tui.RepoDef{{URL: src, Dir: "api", Copy: []string{".env"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(tui.ProjectDef{Name: "demo", Repos: []tui.RepoDef{{URL: src}}}); err == nil {
		t.Error("creating an existing project must fail")
	}
	if err := s.NewTask("demo", "t1", []tui.TaskPick{{Repo: "api"}}); err != nil {
		t.Fatal(err)
	}
	ws, err := s.Workspace("demo", "t1")
	if err != nil || ws != "demo-t1" {
		t.Fatalf("workspace = %q, %v", ws, err)
	}
	info, err := s.Detail("demo")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "demo" || len(info.Repos) != 1 || !info.Repos[0].Cloned || info.Repos[0].Branch != "main" ||
		len(info.Tasks) != 1 || info.Tasks[0].Name != "t1" || info.Tasks[0].Repos[0] != "api" {
		t.Errorf("detail = %+v", info)
	}
	if names, _ := s.List(); len(names) != 1 || names[0] != "demo" {
		t.Errorf("list = %v", names)
	}
}

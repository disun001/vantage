package project

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/git"
)

// fixtureRepo makes a one-commit repo on main and returns its path.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestSyncClonesMissingAndSkipsExisting(t *testing.T) {
	a, b := fixtureRepo(t), fixtureRepo(t)
	root := t.TempDir()
	p := Project{Name: "stack", Repos: []Repo{{URL: a, Dir: "api"}, {URL: b, Dir: "web"}}}

	var out bytes.Buffer
	if err := Sync(root, p, git.Client{}, &out); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"api", "web"} {
		if _, err := os.Stat(filepath.Join(root, "stack", d, ".git")); err != nil {
			t.Errorf("%s not cloned: %v", d, err)
		}
	}

	out.Reset()
	if err := Sync(root, p, git.Client{}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "already cloned") != 2 {
		t.Errorf("second sync should skip both:\n%s", out.String())
	}
}

func TestSyncNeverClonesOverPlainDir(t *testing.T) {
	src := fixtureRepo(t)
	root := t.TempDir()
	plain := filepath.Join(root, "stack", "api")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(plain, "keep.txt")
	if err := os.WriteFile(keep, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := Project{Name: "stack", Repos: []Repo{{URL: src, Dir: "api"}}}
	err := Sync(root, p, git.Client{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not a git repo") {
		t.Fatalf("want not-a-git-repo error, got %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("existing files must survive")
	}
}

func TestSyncContinuesAfterFailure(t *testing.T) {
	good := fixtureRepo(t)
	root := t.TempDir()
	p := Project{Name: "stack", Repos: []Repo{
		{URL: filepath.Join(t.TempDir(), "missing"), Dir: "bad"},
		{URL: good, Dir: "good"},
	}}
	err := Sync(root, p, git.Client{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("want error naming bad, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "stack", "good", ".git")); err != nil {
		t.Error("good repo should still be cloned")
	}
}

func TestCheck(t *testing.T) {
	src := fixtureRepo(t)
	root := t.TempDir()
	p := Project{Name: "stack", Repos: []Repo{{URL: src, Dir: "api"}, {URL: src, Dir: "later"}}}
	if err := Sync(root, Project{Name: "stack", Repos: p.Repos[:1]}, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "stack", "api", "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := Check(root, p, git.Client{})
	if len(got) != 2 {
		t.Fatalf("got %d results", len(got))
	}
	if !got[0].Cloned || got[0].Err != nil || len(got[0].Warnings) != 1 || got[0].Warnings[0] != "uncommitted changes" {
		t.Errorf("api = %+v", got[0])
	}
	if got[1].Cloned || got[1].Err != nil {
		t.Errorf("later should be reported as not cloned: %+v", got[1])
	}
}

func TestWarnings(t *testing.T) {
	cases := []struct {
		name string
		st   git.Status
		want int
	}{
		{"pristine", git.Status{Branch: "main", Default: "main"}, 0},
		{"unknown default", git.Status{Branch: "x"}, 0},
		{"off default", git.Status{Branch: "feat", Default: "main"}, 1},
		{"detached", git.Status{Default: "main"}, 1},
		{"dirty and off", git.Status{Branch: "feat", Default: "main", Dirty: true}, 2},
	}
	for _, c := range cases {
		if got := warnings(c.st); len(got) != c.want {
			t.Errorf("%s: warnings = %v", c.name, got)
		}
	}
}

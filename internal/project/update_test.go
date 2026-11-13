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

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestUpdate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, src, clone string)
		want   string
	}{
		{"fast-forwards", func(t *testing.T, src, _ string) {
			gitRun(t, src, "commit", "-q", "--allow-empty", "-m", "up")
		}, "updated"},
		{"up to date", func(*testing.T, string, string) {}, "up to date"},
		{"dirty is left alone", func(t *testing.T, src, clone string) {
			gitRun(t, src, "commit", "-q", "--allow-empty", "-m", "up")
			if err := os.WriteFile(filepath.Join(clone, "x"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "uncommitted changes"},
		{"off default is left alone", func(t *testing.T, src, clone string) {
			gitRun(t, src, "commit", "-q", "--allow-empty", "-m", "up")
			gitRun(t, clone, "checkout", "-q", "-b", "feature")
		}, "not the default branch"},
		{"diverged is left alone", func(t *testing.T, src, clone string) {
			gitRun(t, src, "commit", "-q", "--allow-empty", "-m", "up")
			gitRun(t, clone, "commit", "-q", "--allow-empty", "-m", "local")
		}, "diverged"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := fixtureRepo(t)
			root := t.TempDir()
			p := Project{Name: "stack", Repos: []Repo{{URL: src, Dir: "api"}}}
			if err := Sync(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			clone := p.RepoPath(root, p.Repos[0])
			c.mutate(t, src, clone)

			var out bytes.Buffer
			if err := Update(root, p, git.Client{}, &out); err != nil {
				t.Fatalf("%v\n%s", err, out.String())
			}
			if !strings.Contains(out.String(), c.want) {
				t.Errorf("want %q in:\n%s", c.want, out.String())
			}
		})
	}
}

func TestUpdateActuallyAdvances(t *testing.T) {
	src := fixtureRepo(t)
	root := t.TempDir()
	p := Project{Name: "stack", Repos: []Repo{{URL: src, Dir: "api"}}}
	if err := Sync(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, "commit", "-q", "--allow-empty", "-m", "up")
	if err := Update(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	st, err := git.Client{}.Status(p.RepoPath(root, p.Repos[0]))
	if err != nil || st.Behind != 0 {
		t.Errorf("still behind after update: %+v, %v", st, err)
	}
}

func TestUpdateSkipsNotCloned(t *testing.T) {
	p := Project{Name: "stack", Repos: []Repo{{URL: "/nowhere", Dir: "api"}}}
	var out bytes.Buffer
	if err := Update(t.TempDir(), p, git.Client{}, &out); err != nil || !strings.Contains(out.String(), "not cloned") {
		t.Errorf("err=%v out=%q", err, out.String())
	}
}

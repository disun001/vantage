package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/git"
)

// taskFixture syncs a two-repo project (api, web) and returns its pieces.
func taskFixture(t *testing.T) (root string, p Project, srcs [2]string) {
	t.Helper()
	srcs = [2]string{fixtureRepo(t), fixtureRepo(t)}
	root = t.TempDir()
	p = Project{Name: "stack", Repos: []Repo{
		{URL: srcs[0], Dir: "api", Copy: []string{".env", "cfg/*.json", "*/config", "link"}},
		{URL: srcs[1], Dir: "web"},
	}}
	if err := Sync(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return root, p, srcs
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool { _, err := os.Lstat(path); return err == nil }

func TestValidTaskName(t *testing.T) {
	for name, ok := range map[string]bool{
		"fix-login": true, "a_1": true, "": false, "-x": false, "a/b": false, "..": false, "a b": false, "a.b": false,
	} {
		if got := ValidTaskName(name) == nil; got != ok {
			t.Errorf("ValidTaskName(%q) = %v, want %v", name, got, ok)
		}
	}
}

func TestValidBranch(t *testing.T) {
	for b, ok := range map[string]bool{
		"main": true, "release/1.2": true, "feat_x-y": true, "": false, "-x": false, "a..b": false,
		"a//b": false, "a/": false, "a.lock": false, "a/.b": false, "a b": false, "x;y": false,
	} {
		if got := validBranch(b) == nil; got != ok {
			t.Errorf("validBranch(%q) = %v, want %v", b, got, ok)
		}
	}
}

func TestNewTaskCreatesWorktreesAndCopies(t *testing.T) {
	root, p, _ := taskFixture(t)
	api := p.RepoPath(root, p.Repos[0])
	write(t, filepath.Join(api, ".env"), "SECRET=1")
	write(t, filepath.Join(api, "cfg", "a.json"), "{}")
	if err := os.Symlink("/etc/hosts", filepath.Join(api, "link")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := NewTask(root, p, git.Client{}, "t1", nil, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	for _, r := range p.Repos {
		if !exists(filepath.Join(p.TaskRepoPath(root, "t1", r), ".git")) {
			t.Errorf("%s worktree missing", r.DirName())
		}
	}
	wt := p.TaskRepoPath(root, "t1", p.Repos[0])
	for _, f := range []string{".env", "cfg/a.json"} {
		if !exists(filepath.Join(wt, f)) {
			t.Errorf("%s should be copied", f)
		}
	}
	if exists(filepath.Join(wt, "link")) {
		t.Error("symlink must not be copied")
	}
	if strings.Contains(out.String(), "api/.git") {
		t.Errorf("nothing under .git may be copied or mentioned:\n%s", out.String())
	}
	if strings.Contains(out.String(), "SECRET") {
		t.Error("file contents must never be printed")
	}
	if !git.Client.RefExists(git.Client{}, api, "refs/heads/t1") {
		t.Error("branch t1 missing")
	}
}

func TestNewTaskSelectsReposAndBase(t *testing.T) {
	root, p, srcs := taskFixture(t)
	gitRun(t, srcs[1], "branch", "develop")
	web := p.RepoPath(root, p.Repos[1])
	if err := (git.Client{}).Fetch(web); err != nil {
		t.Fatal(err)
	}
	err := NewTask(root, p, git.Client{}, "t2", []TaskRepo{{Dir: "web", Base: "develop"}}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if exists(p.TaskRepoPath(root, "t2", p.Repos[0])) {
		t.Error("api was not selected")
	}
	if !exists(filepath.Join(p.TaskRepoPath(root, "t2", p.Repos[1]), ".git")) {
		t.Error("web worktree missing")
	}
}

func TestNewTaskFlat(t *testing.T) {
	src := fixtureRepo(t)
	root := t.TempDir()
	p := Project{Name: "solo", Flat: true, Repos: []Repo{{URL: src}}}
	if err := Sync(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := NewTask(root, p, git.Client{}, "t1", nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(root, ".tasks", "solo", "t1", ".git")) {
		t.Error("flat worktree should be the task dir")
	}
	if err := RemoveTask(root, p, git.Client{}, "t1", false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, ".tasks", "solo")) {
		t.Error("empty task dirs should be cleaned up")
	}
}

func TestNewTaskRefusals(t *testing.T) {
	root, p, _ := taskFixture(t)
	g := git.Client{}
	gitRun(t, p.RepoPath(root, p.Repos[1]), "branch", "taken")
	cases := []struct {
		name  string
		task  string
		repos []TaskRepo
		want  string
	}{
		{"bad name", "a b", nil, "invalid task name"},
		{"unknown repo", "t", []TaskRepo{{Dir: "nope"}}, "unknown repo"},
		{"duplicate repo", "t", []TaskRepo{{Dir: "api"}, {Dir: "api"}}, "twice"},
		{"bad base", "t", []TaskRepo{{Dir: "api", Base: "--x"}}, "invalid branch"},
		{"missing base", "t", []TaskRepo{{Dir: "api", Base: "nope"}}, "not found"},
		{"branch exists", "taken", nil, "already exists"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := NewTask(root, p, g, c.task, c.repos, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
			if exists(p.TaskPath(root, c.task)) {
				t.Error("a refused task must leave nothing behind")
			}
		})
	}
	if err := NewTask(root, p, g, "ok", nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := NewTask(root, p, g, "ok", nil, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate task: %v", err)
	}
}

func TestRollbackUndoesWorktrees(t *testing.T) {
	root, p, _ := taskFixture(t)
	g := git.Client{}
	plan, err := planTask(root, p, g, "r1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := createWorktrees(root, p, g, "r1", plan, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	rollback(g, "r1", plan, p.TaskPath(root, "r1"), &bytes.Buffer{})
	if exists(p.TaskPath(root, "r1")) {
		t.Error("task dir should be gone")
	}
	if g.RefExists(p.RepoPath(root, p.Repos[0]), "refs/heads/r1") {
		t.Error("branch should be gone")
	}
}

func TestRemoveTask(t *testing.T) {
	g := git.Client{}
	setup := func(t *testing.T) (string, Project) {
		root, p, _ := taskFixture(t)
		if err := NewTask(root, p, g, "t", nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		return root, p
	}
	t.Run("clean task is removed with its merged branch", func(t *testing.T) {
		root, p := setup(t)
		if err := RemoveTask(root, p, g, "t", false, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if exists(p.TaskPath(root, "t")) || g.RefExists(p.RepoPath(root, p.Repos[0]), "refs/heads/t") {
			t.Error("task dir and branch should be gone")
		}
	})
	t.Run("dirty worktree blocks, nothing touched", func(t *testing.T) {
		root, p := setup(t)
		write(t, filepath.Join(p.TaskRepoPath(root, "t", p.Repos[1]), "wip.txt"), "x")
		err := RemoveTask(root, p, g, "t", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "web: uncommitted") {
			t.Fatalf("want refusal naming web, got %v", err)
		}
		for _, r := range p.Repos {
			if !exists(p.TaskRepoPath(root, "t", r)) {
				t.Errorf("%s must be untouched (all or nothing)", r.DirName())
			}
		}
	})
	t.Run("unmerged commit blocks; force removes but keeps the branch", func(t *testing.T) {
		root, p := setup(t)
		wt := p.TaskRepoPath(root, "t", p.Repos[0])
		gitRun(t, wt, "commit", "-q", "--allow-empty", "-m", "work")
		err := RemoveTask(root, p, g, "t", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "unpushed") || !strings.Contains(err.Error(), "not merged") {
			t.Fatalf("want unpushed and unmerged, got %v", err)
		}
		var out bytes.Buffer
		if err := RemoveTask(root, p, g, "t", true, &out); err != nil {
			t.Fatal(err)
		}
		if exists(p.TaskPath(root, "t")) {
			t.Error("task dir should be gone")
		}
		if !g.RefExists(p.RepoPath(root, p.Repos[0]), "refs/heads/t") || !strings.Contains(out.String(), "kept") {
			t.Errorf("unmerged branch must be kept:\n%s", out.String())
		}
	})
	t.Run("missing task", func(t *testing.T) {
		root, p, _ := taskFixture(t)
		if err := RemoveTask(root, p, g, "nope", false, &bytes.Buffer{}); err == nil {
			t.Error("want not found")
		}
	})
}

func TestListTasks(t *testing.T) {
	root, p, _ := taskFixture(t)
	g := git.Client{}
	for _, n := range []string{"b", "a"} {
		if err := NewTask(root, p, g, n, []TaskRepo{{Dir: "web"}}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ListTasks(root, p)
	if err != nil || strings.Join(got, ",") != "a,b" {
		t.Errorf("ListTasks = %v, %v", got, err)
	}
	if rs := TaskRepos(root, p, "a"); len(rs) != 1 || rs[0].DirName() != "web" {
		t.Errorf("TaskRepos = %v", rs)
	}
}

func TestCopySkipsTrackedFiles(t *testing.T) {
	src := fixtureRepo(t)
	write(t, filepath.Join(src, "tracked.env"), "x")
	gitRun(t, src, "add", ".")
	gitRun(t, src, "commit", "-q", "-m", "add")
	root := t.TempDir()
	p := Project{Name: "stack", Repos: []Repo{{URL: src, Dir: "api", Copy: []string{"tracked.env"}}}}
	if err := Sync(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := NewTask(root, p, git.Client{}, "t", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "tracked by git") || strings.Contains(out.String(), "copied  ") {
		t.Errorf("tracked file should warn, not copy:\n%s", out.String())
	}
}

func TestCopyNeverOverwrites(t *testing.T) {
	root, p, _ := taskFixture(t)
	write(t, filepath.Join(p.RepoPath(root, p.Repos[0]), ".env"), "new")
	wt := p.TaskRepoPath(root, "t", p.Repos[0])
	if err := copyFiles(git.Client{}, "api", p.RepoPath(root, p.Repos[0]), wt, []string{".env"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, ".env"), "mine")
	if err := copyFiles(git.Client{}, "api", p.RepoPath(root, p.Repos[0]), wt, []string{".env"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(wt, ".env")); string(b) != "mine" {
		t.Errorf("existing file overwritten: %q", b)
	}
}

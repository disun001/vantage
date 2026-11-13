package project

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/git"
)

func TestRemoveProject(t *testing.T) {
	g := git.Client{}
	setup := func(t *testing.T) (string, Project) {
		root, p, _ := taskFixture(t)
		t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
		t.Setenv("VANTAGE_PROJECTS_DIR", root)
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
		if err := NewTask(root, p, g, "t", nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		return root, p
	}
	t.Run("removes tasks, clones and definition", func(t *testing.T) {
		root, p := setup(t)
		if err := Remove(root, p, g, false, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		file, _ := PathOf(p.Name)
		for _, path := range []string{file, p.Path(root), p.TaskPath(root, "t")} {
			if exists(path) {
				t.Errorf("%s should be gone", path)
			}
		}
		if names, _ := List(); len(names) != 0 {
			t.Errorf("List = %v", names)
		}
	})
	t.Run("dirty task blocks, nothing touched", func(t *testing.T) {
		root, p := setup(t)
		write(t, filepath.Join(p.TaskRepoPath(root, "t", p.Repos[1]), "wip.txt"), "x")
		err := Remove(root, p, g, false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "task t web: uncommitted") {
			t.Fatalf("want refusal, got %v", err)
		}
		file, _ := PathOf(p.Name)
		if !exists(file) || !exists(p.RepoPath(root, p.Repos[0])) || !exists(p.TaskPath(root, "t")) {
			t.Error("a refusal must touch nothing")
		}
	})
	t.Run("dirty main clone blocks, force removes", func(t *testing.T) {
		root, p := setup(t)
		write(t, filepath.Join(p.RepoPath(root, p.Repos[0]), "wip.txt"), "x")
		err := Remove(root, p, g, false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "api: uncommitted") {
			t.Fatalf("want refusal, got %v", err)
		}
		if err := Remove(root, p, g, true, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if exists(p.Path(root)) {
			t.Error("project dir should be gone")
		}
	})
}

func TestInside(t *testing.T) {
	for path, want := range map[string]bool{"/r/a": true, "/r/a/b": true, "/r": false, "/x": false, "/r/../x": false} {
		if got := inside("/r", path); got != want {
			t.Errorf("inside(/r, %s) = %v", path, got)
		}
	}
}

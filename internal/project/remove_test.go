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
		root, p := fixture(t)
		t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
		t.Setenv("VANTAGE_PROJECTS_DIR", root)
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
		return root, p
	}
	t.Run("removes clones and definition", func(t *testing.T) {
		root, p := setup(t)
		if err := Remove(root, p, g, false, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		file, _ := PathOf(p.Name)
		for _, path := range []string{file, p.Path(root)} {
			if exists(path) {
				t.Errorf("%s should be gone", path)
			}
		}
		if names, _ := List(); len(names) != 0 {
			t.Errorf("List = %v", names)
		}
	})
	t.Run("dirty main clone blocks, force removes", func(t *testing.T) {
		root, p := setup(t)
		write(t, filepath.Join(p.RepoPath(root, p.Repos[0]), "wip.txt"), "x")
		err := Remove(root, p, g, false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "api: uncommitted") {
			t.Fatalf("want refusal, got %v", err)
		}
		if file, _ := PathOf(p.Name); !exists(file) || !exists(p.RepoPath(root, p.Repos[1])) {
			t.Error("a refusal must touch nothing")
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

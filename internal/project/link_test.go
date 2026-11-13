package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/git"
	"github.com/disun001/vantage/internal/workspace"
)

// linkFixture is a synced project with one task and an isolated config dir
// that holds the project file.
func linkFixture(t *testing.T) (root string, p Project) {
	t.Helper()
	root, p, _ = taskFixture(t)
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	t.Setenv("VANTAGE_PROJECTS_DIR", root)
	def := "[[repo]]\nurl = \"" + p.Repos[0].URL + "\"\ndir = \"api\"\n\n[[repo]]\nurl = \"" + p.Repos[1].URL + "\"\ndir = \"web\"\n"
	write(t, filepath.Join(cfg, "projects", "stack.toml"), def)
	if err := NewTask(root, p, git.Client{}, "t1", nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return root, p
}

func TestApplyRoot(t *testing.T) {
	root, _ := linkFixture(t)
	cases := []struct {
		name string
		ws   workspace.Workspace
		want string
	}{
		{"no project", workspace.Workspace{Name: "w"}, ""},
		{"project", workspace.Workspace{Name: "w", Project: "stack"}, filepath.Join(root, "stack")},
		{"task", workspace.Workspace{Name: "w", Project: "stack", Task: "t1"}, filepath.Join(root, ".tasks", "stack", "t1")},
		{"explicit root wins", workspace.Workspace{Name: "w", Project: "stack", Task: "t1", Root: "/x"}, "/x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ApplyRoot(&c.ws); err != nil || c.ws.Root != c.want {
				t.Errorf("root = %q, %v; want %q", c.ws.Root, err, c.want)
			}
		})
	}
	ws := workspace.Workspace{Name: "w", Project: "nope"}
	if err := ApplyRoot(&ws); err == nil {
		t.Error("unknown project should fail")
	}
}

func TestEnsureTaskWorkspace(t *testing.T) {
	root, p := linkFixture(t)
	name, created, err := EnsureTaskWorkspace(root, p, "t1")
	if err != nil || !created || name != "stack-t1" {
		t.Fatalf("got %q created=%v err=%v", name, created, err)
	}
	path, _ := workspace.PathOf(name)
	ws, err := workspace.Load(path)
	if err != nil || ws.Project != "stack" || ws.Task != "t1" || ws.Root != "" {
		t.Fatalf("saved workspace = %+v, %v (root must stay derived)", ws, err)
	}
	if len(ws.Windows) != 1 || len(ws.Windows[0].Panes) != 1 || ws.Windows[0].Panes[0].Cmd != "" {
		t.Errorf("want one window, one bare pane: %+v", ws.Windows)
	}

	// Renamed file is still found by its keys.
	renamed, _ := workspace.PathOf("my-name")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	name, created, err = EnsureTaskWorkspace(root, p, "t1")
	if err != nil || created || name != "stack-t1" {
		t.Errorf("renamed workspace: got %q created=%v err=%v", name, created, err)
	}
	if got, ok, _ := FindTaskWorkspace("stack", "t1"); !ok || got != "stack-t1" {
		t.Errorf("find by keys: %q %v", got, ok)
	}

	if _, _, err := EnsureTaskWorkspace(root, p, "nope"); err == nil {
		t.Error("missing task should fail")
	}
}

func TestRemoveTaskWarnsAboutWorkspace(t *testing.T) {
	root, p := linkFixture(t)
	if _, _, err := EnsureTaskWorkspace(root, p, "t1"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RemoveTask(root, p, git.Client{}, "t1", false, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "workspace \"stack-t1\" still points") {
		t.Errorf("want workspace warning:\n%s", out.String())
	}
	if path, _ := workspace.PathOf("stack-t1"); !exists(path) {
		t.Error("the workspace file must not be deleted automatically")
	}
}

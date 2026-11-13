package project

import (
	"path/filepath"
	"testing"

	"github.com/disun001/vantage/internal/workspace"
)

func TestApplyRoot(t *testing.T) {
	root, p := fixture(t)
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	t.Setenv("VANTAGE_PROJECTS_DIR", root)
	def := "[[repo]]\nurl = \"" + p.Repos[0].URL + "\"\ndir = \"api\"\n\n[[repo]]\nurl = \"" + p.Repos[1].URL + "\"\ndir = \"web\"\n"
	write(t, filepath.Join(cfg, "projects", "stack.toml"), def)
	cases := []struct {
		name string
		ws   workspace.Workspace
		want string
	}{
		{"no project", workspace.Workspace{Name: "w"}, ""},
		{"project", workspace.Workspace{Name: "w", Project: "stack"}, filepath.Join(root, "stack")},
		{"explicit root wins", workspace.Workspace{Name: "w", Project: "stack", Root: "/x"}, "/x"},
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

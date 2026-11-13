package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/disun001/vantage/internal/git"
)

// fixture syncs a two-repo project (api, web) and returns its pieces.
func fixture(t *testing.T) (root string, p Project) {
	t.Helper()
	root = t.TempDir()
	p = Project{Name: "stack", Repos: []Repo{{URL: fixtureRepo(t), Dir: "api"}, {URL: fixtureRepo(t), Dir: "web"}}}
	if err := Sync(root, p, git.Client{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return root, p
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

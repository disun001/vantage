package manage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// testManager returns a manager bound to a throwaway tmux server and an
// empty config dir. Tests must never use the default server.
func testManager(t *testing.T) Manager {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	return Manager{Client: c}
}

func saved(t *testing.T, name string) bool {
	t.Helper()
	ok, err := workspace.Exists(name)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestCreateSavesAndStarts(t *testing.T) {
	m := testManager(t)
	if err := m.Create("proj"); err != nil {
		t.Fatal(err)
	}
	if !m.Client.HasSession("proj") || !saved(t, "proj") {
		t.Fatal("want a live session and a saved file")
	}
	for _, bad := range []string{"", "a b", "../x", "a/b", "-x"} {
		if err := m.Create(bad); err == nil {
			t.Errorf("Create(%q) succeeded", bad)
		}
	}
	if err := m.Create("proj"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate: %v", err)
	}
}

func TestCreateRefusesLiveSessionName(t *testing.T) {
	m := testManager(t)
	if _, err := m.Client.NewSession("busy", "", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := m.Create("busy"); err == nil {
		t.Fatal("want an error")
	}
	if saved(t, "busy") {
		t.Error("a refused create must not write a file")
	}
}

func TestRenameMovesSessionAndFile(t *testing.T) {
	m := testManager(t)
	if err := m.Create("old"); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename("old", "fresh"); err != nil {
		t.Fatal(err)
	}
	if m.Client.HasSession("old") || !m.Client.HasSession("fresh") || saved(t, "old") || !saved(t, "fresh") {
		t.Fatal("old name must be gone and the new one present, live and saved")
	}
	path, _ := workspace.PathOf("fresh")
	if ws, err := workspace.Load(path); err != nil || ws.Name != "fresh" {
		t.Fatalf("load: %+v, %v", ws, err)
	}
}

func TestRenameSavedOnlyKeepsComments(t *testing.T) {
	m := testManager(t)
	path, _ := workspace.PathOf("cold")
	body := "# my notes\nname = \"cold\"\nroot = \"~\"\n\n[[window]]\nname = \"cold\"\n[[window.pane]]\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename("cold", "warm"); err != nil {
		t.Fatal(err)
	}
	newPath, _ := workspace.PathOf("warm")
	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(body, `name = "cold"`+"\nroot", `name = "warm"`+"\nroot", 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if saved(t, "cold") {
		t.Error("old file must be gone")
	}
}

func TestRenameConflictsLeaveEverythingAlone(t *testing.T) {
	m := testManager(t)
	for _, n := range []string{"a", "b"} {
		if err := m.Create(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Rename("a", "b"); err == nil {
		t.Fatal("want a conflict error")
	}
	if err := m.Rename("a", "bad name"); err == nil {
		t.Fatal("want an invalid-name error")
	}
	if err := m.Rename("ghost", "c"); err == nil {
		t.Fatal("want an error for a missing source")
	}
	if !m.Client.HasSession("a") || !saved(t, "a") {
		t.Error("a must be untouched")
	}
}

func TestDeleteRemovesFileAndSession(t *testing.T) {
	m := testManager(t)
	if err := m.Create("gone"); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete("gone"); err != nil {
		t.Fatal(err)
	}
	if saved(t, "gone") || m.Client.HasSession("gone") {
		t.Fatal("file and session must both go")
	}
	if err := m.Delete("gone"); err == nil {
		t.Error("deleting twice must fail")
	}
	// A session with no file is killed too.
	if _, err := m.Client.NewSession("loose", "", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete("loose"); err != nil || m.Client.HasSession("loose") {
		t.Errorf("live-only delete: %v", err)
	}
	if err := m.Delete("../escape"); err == nil {
		t.Error("a path must not be a workspace name")
	}
}

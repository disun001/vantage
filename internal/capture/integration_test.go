package capture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/restore"
	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

func testClient(t *testing.T) tmux.Client {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	return c
}

// A session built by restore, captured, saved, loaded and restored under a
// new name must keep its shape.
func TestCaptureRoundTrip(t *testing.T) {
	c := testClient(t)
	dir := t.TempDir()
	src := workspace.Workspace{
		Name: "src",
		Root: dir,
		Windows: []workspace.Window{
			{Name: "one", Layout: "tiled", Panes: []workspace.Pane{{}, {Split: "v"}, {}}},
			{Name: "two", Panes: []workspace.Pane{{}}},
		},
	}
	if _, err := restore.Restore(c, src); err != nil {
		t.Fatal(err)
	}

	got, err := Session(c, "src", "copy", PS)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 2 || len(got.Windows[0].Panes) != 3 || got.Windows[0].Name != "one" {
		t.Fatalf("captured shape: %+v", got)
	}

	path := filepath.Join(t.TempDir(), "copy.toml")
	if err := workspace.Save(path, got, false); err != nil {
		t.Fatal(err)
	}
	loaded, err := workspace.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := restore.Restore(c, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if res.Windows != 2 || res.Panes != 4 {
		t.Fatalf("restored copy: %+v", res)
	}
}

func TestSessionMissing(t *testing.T) {
	c := testClient(t)
	if _, err := restore.Restore(c, workspace.Workspace{Name: "x", Windows: []workspace.Window{{Panes: []workspace.Pane{{}}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Session(c, "nope", "", nil); err == nil {
		t.Fatal("want error for a missing session")
	}
}

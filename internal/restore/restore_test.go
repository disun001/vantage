package restore

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// testClient returns a client bound to a throwaway tmux server. Tests must
// never use the default server (see internal/tmux safety rule).
func testClient(t *testing.T) tmux.Client {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	return c
}

func TestRestoreBuildsWindowsAndPanes(t *testing.T) {
	c := testClient(t)
	dir := t.TempDir()
	ws := workspace.Workspace{
		Name: "demo",
		Root: dir,
		Windows: []workspace.Window{
			{Name: "one", Layout: "tiled", Panes: []workspace.Pane{{}, {Split: "v"}, {}}},
			{Name: "two", Panes: []workspace.Pane{{}}},
		},
	}
	res, err := Restore(c, ws)
	if err != nil {
		t.Fatal(err)
	}
	if res.Existed || res.Windows != 2 || res.Panes != 4 {
		t.Fatalf("result = %+v", res)
	}
	lines, err := c.ListPanes("#{session_name}|#{window_name}|#{pane_current_path}")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 4 {
		t.Fatalf("want 4 panes, got %v", lines)
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "demo|") {
			t.Errorf("unexpected session in %q", l)
		}
	}
}

func TestRestoreIsIdempotent(t *testing.T) {
	c := testClient(t)
	ws := workspace.Workspace{Name: "demo", Windows: []workspace.Window{{Panes: []workspace.Pane{{}}}}}
	if _, err := Restore(c, ws); err != nil {
		t.Fatal(err)
	}
	res, err := Restore(c, ws)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Existed {
		t.Errorf("second restore should report Existed, got %+v", res)
	}
	lines, _ := c.ListPanes("#{pane_id}")
	if len(lines) != 1 {
		t.Errorf("second restore must not add panes, got %d", len(lines))
	}
}

func TestRestoreRunsOnlyAutorunCommands(t *testing.T) {
	c := testClient(t)
	dir := t.TempDir()
	ws := workspace.Workspace{
		Name: "demo",
		Root: dir,
		Windows: []workspace.Window{{Panes: []workspace.Pane{
			{Cmd: "touch ran-auto", Autorun: true},
			{Cmd: "touch ran-off"},
		}}},
	}
	if _, err := Restore(c, ws); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir + "/ran-auto"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := os.Stat(dir + "/ran-auto"); err != nil {
		t.Error("autorun command did not run")
	}
	if _, err := os.Stat(dir + "/ran-off"); err == nil {
		t.Error("command without autorun ran")
	}
}

func TestRestoreCleansUpOnFailure(t *testing.T) {
	c := testClient(t)
	ws := workspace.Workspace{
		Name: "demo",
		Windows: []workspace.Window{{
			Layout: "not-a-layout",
			Panes:  []workspace.Pane{{}, {}},
		}},
	}
	if _, err := Restore(c, ws); err == nil {
		t.Fatal("expected layout error")
	}
	if c.HasSession("demo") {
		t.Error("half-built session was not removed")
	}
}

func TestKillServerRefusesDefault(t *testing.T) {
	if err := (tmux.Client{}).KillServer(); err == nil {
		t.Fatal("KillServer on the default server must be refused")
	}
}

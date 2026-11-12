package hostuse

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
)

func writeWS(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// testFinder binds a Finder to a throwaway tmux server and a temp config dir.
// Tests must never use the default server.
func testFinder(t *testing.T, procs func(string) ([]string, error)) (Finder, string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	dir := filepath.Join(cfg, "workspaces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	return Finder{Client: c, Procs: procs}, dir
}

func TestFindAndClose(t *testing.T) {
	// Every pane looks like it runs ssh to "web", except the ones whose tty
	// is listed in plain.
	plain := map[string]bool{}
	f, dir := testFinder(t, func(tty string) ([]string, error) {
		if plain[tty] {
			return []string{"-zsh"}, nil
		}
		return []string{"ssh web"}, nil
	})
	writeWS(t, dir, "uses", "name = \"uses\"\n[[window]]\nname = \"w\"\n[[window.pane]]\nssh = \"web\"\n")
	writeWS(t, dir, "other", "name = \"other\"\n[[window]]\nname = \"w\"\n[[window.pane]]\nssh = \"db\"\n")
	writeWS(t, dir, "broken", "this is not toml [[[")

	first, err := f.Client.NewSession("s", "w1", os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.Client.NewWindow("s", "w2", os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	panes, err := f.Client.ListAllPanes()
	if err != nil || len(panes) != 2 {
		t.Fatalf("panes %v, %v", panes, err)
	}
	for _, p := range panes {
		if p.Pane == first {
			plain[p.TTY] = true // the first pane is just a shell
		}
	}

	u, err := f.Find("web")
	if err == nil {
		t.Error("the broken workspace file should be reported")
	}
	if !reflect.DeepEqual(u.Workspaces, []string{"uses"}) {
		t.Errorf("workspaces = %v", u.Workspaces)
	}
	if len(u.Panes) != 1 || u.Panes[0].Pane != second {
		t.Fatalf("panes = %+v, want only %s", u.Panes, second)
	}
	if u.Empty() {
		t.Error("Empty on a used host")
	}
	if none, _ := f.Find("nobody"); !none.Empty() {
		t.Errorf("unused alias reports %+v", none)
	}

	if err := f.Close(u); err != nil {
		t.Fatal(err)
	}
	left, _ := f.Client.ListAllPanes()
	if len(left) != 1 || left[0].Pane != first {
		t.Errorf("panes left: %+v", left)
	}
	if err := f.Close(u); err != nil { // already gone: not an error
		t.Errorf("closing closed panes: %v", err)
	}
}

func TestKillPaneRefusesOddIDs(t *testing.T) {
	c := tmux.Client{Socket: "vantage-test-never-started"}
	for _, bad := range []string{"", "web", "%", "%1;x", "-t", "%a"} {
		if err := c.KillPane(bad); err == nil {
			t.Errorf("KillPane(%q) accepted", bad)
		}
	}
}

package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func testClient(t *testing.T) Client {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	c := Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	return c
}

func TestListSessionsAndAttachCmd(t *testing.T) {
	c := testClient(t)
	if ss, err := c.ListSessions(); err != nil || len(ss) != 0 {
		t.Fatalf("no server: got %v, %v", ss, err)
	}
	if _, err := c.NewSession("alpha", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewWindow("alpha", "w2", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	ss, err := c.ListSessions()
	if err != nil || len(ss) != 1 || ss[0].Name != "alpha" || ss[0].Windows != 2 {
		t.Fatalf("got %v, %v", ss, err)
	}
	cmd := c.AttachCmd("alpha")
	for _, kv := range cmd.Env {
		if kv == "TMUX=1" {
			t.Fatal("TMUX leaked")
		}
	}
}

func TestClientTTYUnknownPID(t *testing.T) {
	c := testClient(t)
	if _, err := c.NewSession("s", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ClientTTY(1); err == nil {
		t.Fatal("want error when no client has that pid")
	}
}

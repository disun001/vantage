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

func TestWindowsSelectAndStatusBar(t *testing.T) {
	c := testClient(t)
	if _, err := c.NewSession("alpha", "one", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewWindow("alpha", "tw|o", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	ss, err := c.ListSessions()
	if err != nil || len(ss) != 1 || ss[0].ID == "" {
		t.Fatalf("sessions: %v, %v", ss, err)
	}
	ws, err := c.ListWindows()
	if err != nil || len(ws) != 2 {
		t.Fatalf("windows: %v, %v", ws, err)
	}
	for _, w := range ws {
		if w.SessionID != ss[0].ID || w.Panes != 1 {
			t.Errorf("bad window %+v", w)
		}
	}
	if ws[1].Name != "tw|o" || ws[0].Active == ws[1].Active {
		t.Errorf("name or active flag wrong: %+v", ws)
	}
	if err := c.SelectWindow("alpha", ws[0].Index); err != nil {
		t.Fatal(err)
	}
	if ws, _ = c.ListWindows(); !ws[0].Active || ws[1].Active {
		t.Errorf("select-window did not move the active flag: %+v", ws)
	}
}

func TestStatusLines(t *testing.T) {
	for in, want := range map[string]int{"on": 1, "off": 0, "": 0, "2": 2, "5": 5, "0": 0, "junk": 0} {
		if got := statusLines(in); got != want {
			t.Errorf("statusLines(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"0|0|0":    {},
		"1|0|0":    {Prefix: true},
		"0|1|0":    {Copy: true},
		"0|0|1":    {Sync: true},
		"1|1|1":    {Prefix: true, Copy: true, Sync: true},
		"":         {},
		"1|0":      {},
		"1|0|0|0":  {},
		"yes|0|no": {},
	}
	for in, want := range cases {
		if got := parseMode(in); got != want {
			t.Errorf("parseMode(%q) = %+v, want %+v", in, got, want)
		}
	}
}

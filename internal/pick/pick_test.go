package pick

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
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

func TestSplitRejectsBadInput(t *testing.T) {
	c := tmux.Client{Socket: "unused"} // validation happens before any tmux call
	for _, tt := range []struct {
		dir   string
		alias string
	}{
		{"x", ""},
		{"h", "-oProxyCommand=evil"},
		{"v", "a b"},
		{"h", "a;b"},
	} {
		if _, err := Split(c, "%0", tt.dir, "", Choice{Alias: tt.alias}); err == nil {
			t.Errorf("Split(%q, %q) accepted bad input", tt.dir, tt.alias)
		}
	}
}

func TestSplitLocalAndSSH(t *testing.T) {
	c := testClient(t)
	pane, err := c.NewSession("s", "w", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Split(c, pane, "h", "", Choice{}); err != nil {
		t.Fatal(err)
	}
	// 192.0.2.1 is TEST-NET: never a real host.
	id, err := Split(c, pane, "v", "", Choice{Alias: "192.0.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	panes, err := c.ListPanes("#{pane_id}")
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 3 {
		t.Fatalf("want 3 panes, got %v", panes)
	}
	if id == "" {
		t.Fatal("no pane id returned")
	}
}

func TestChatCommand(t *testing.T) {
	tests := []struct {
		exe, want string
		bad       bool
	}{
		{exe: "/opt/bin/vantage", want: "'/opt/bin/vantage' chat new"},
		{exe: "/a b/it's", want: `'/a b/it'\''s' chat new`},
		{exe: "vantage", bad: true},
		{exe: "", bad: true},
		{exe: "/x\n; rm -rf /", bad: true},
	}
	for _, tt := range tests {
		got, err := chatCommand(tt.exe)
		if (err != nil) != tt.bad || got != tt.want {
			t.Errorf("chatCommand(%q) = %q, %v; want %q (bad=%v)", tt.exe, got, err, tt.want, tt.bad)
		}
	}
}

func TestSplitChatRejectsBadExe(t *testing.T) {
	if _, err := Split(tmux.Client{Socket: "unused"}, "%0", "h", "", Choice{Chat: true, Exe: "relative"}); err == nil {
		t.Error("relative exe accepted")
	}
}

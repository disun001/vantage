package pick

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
)

func TestChoose(t *testing.T) {
	hosts := []string{"web", "db"}
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"\n", "", false},
		{"", "", false},
		{"0\n", "", false},
		{"l\n", "", false},
		{"1\n", "web", false},
		{"2\n", "db", false},
		{"db\n", "db", false},
		{"3\n", "", true},
		{"-1\n", "", true},
		{"nope\n", "", true},
		{"-oProxyCommand=x\n", "", true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			var out bytes.Buffer
			got, err := Choose(strings.NewReader(tt.in), &out, hosts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Alias != tt.want {
				t.Fatalf("alias = %q, want %q", got.Alias, tt.want)
			}
		})
	}
}

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

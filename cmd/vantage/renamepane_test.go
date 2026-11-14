package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/tmux"
)

func renameClient(t *testing.T) (tmux.Client, string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	pane, err := c.NewSession("r", "w", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return c, pane
}

func TestRenamePane(t *testing.T) {
	c, pane := renameClient(t)
	tests := []struct {
		name    string
		args    []string
		stdin   string
		want    string
		wantErr bool
	}{
		{"argument", []string{"api"}, "", "api", false},
		{"typed line", nil, "web server\n", "web server", false},
		{"typed without newline", nil, "tail", "tail", false},
		{"empty clears", []string{""}, "", "", false},
		{"typed empty clears", nil, "\n", "", false},
		{"cancelled input", nil, "", "keep", true},
		{"format injection", []string{"#(touch /tmp/x)"}, "", "keep", true},
		{"typed injection", nil, "#{pane_id}\n", "keep", true},
		{"too long", []string{strings.Repeat("a", 41)}, "", "keep", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.SetPaneTitle(pane, "keep"); err != nil {
				t.Fatal(err)
			}
			err := renamePane(c, pane, tt.args, strings.NewReader(tt.stdin), io.Discard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got, _ := c.PaneTitle(pane); got != tt.want {
				t.Errorf("title = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenamePaneRejectsBadPane(t *testing.T) {
	var out strings.Builder
	if err := runRenamePane([]string{"--pane", "%1;kill-server", "x"}, &out); err == nil {
		t.Fatal("want error for a bad pane id")
	}
}

package capture

import (
	"errors"
	"testing"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

func fakeProcs(lines ...string) Lister {
	return func(string) ([]string, error) { return lines, nil }
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		command string
		procs   Lister
		want    workspace.Pane
	}{
		{"idle shell", "zsh", nil, workspace.Pane{}},
		{"login shell", "-zsh", nil, workspace.Pane{}},
		{"plain ssh alias", "ssh", fakeProcs("ssh prod"), workspace.Pane{SSH: "prod"}},
		{"ssh with port is a command", "ssh", fakeProcs("ssh -p 2222 prod"), workspace.Pane{Cmd: "ssh -p 2222 prod"}},
		{"ssh user@host is an alias-safe token", "ssh", fakeProcs("ssh me@host"), workspace.Pane{SSH: "me@host"}},
		{"ssh option-like arg is a command", "ssh", fakeProcs("ssh -oProxyCommand=x"), workspace.Pane{Cmd: "ssh -oProxyCommand=x"}},
		{"editor keeps args, no autorun", "nvim", fakeProcs("nvim main.go"), workspace.Pane{Cmd: "nvim main.go"}},
		{"ps fails: name only", "nvim", func(string) ([]string, error) { return nil, errors.New("boom") }, workspace.Pane{Cmd: "nvim"}},
		{"no match: name only", "nvim", fakeProcs("something else"), workspace.Pane{Cmd: "nvim"}},
		{"no lister: name only", "htop", nil, workspace.Pane{Cmd: "htop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tmux.PaneInfo{Command: tt.command, TTY: "/dev/ttys001"}, tt.procs)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
			if got.Autorun {
				t.Error("capture must never set autorun")
			}
		})
	}
}

func TestBuildGroupsWindowsAndContractsHome(t *testing.T) {
	panes := []tmux.PaneInfo{
		{Window: 0, WindowName: "code", Layout: "L1", CWD: "/home/me/proj", Command: "zsh"},
		{Window: 0, WindowName: "code", Layout: "L1", CWD: "/home/me", Command: "zsh"},
		{Window: 1, WindowName: "logs", Layout: "L2", CWD: "/var/log", Command: "zsh"},
	}
	ws, err := build("demo", "/home/me", panes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Windows) != 2 || len(ws.Windows[0].Panes) != 2 {
		t.Fatalf("shape: %+v", ws.Windows)
	}
	for i, want := range []string{"~/proj", "~", "/var/log"} {
		var got string
		if i < 2 {
			got = ws.Windows[0].Panes[i].CWD
		} else {
			got = ws.Windows[1].Panes[0].CWD
		}
		if got != want {
			t.Errorf("cwd %d = %q, want %q", i, got, want)
		}
	}
}

func TestBuildRejectsBadName(t *testing.T) {
	_, err := build("bad name", "/h", []tmux.PaneInfo{{Command: "zsh"}}, nil)
	if err == nil {
		t.Fatal("want error for invalid name")
	}
}

func TestContractHome(t *testing.T) {
	tests := []struct{ path, home, want string }{
		{"/home/me", "/home/me", "~"},
		{"/home/me/x", "/home/me", "~/x"},
		{"/home/meow", "/home/me", "/home/meow"},
		{"/x", "", "/x"},
	}
	for _, tt := range tests {
		if got := contractHome(tt.path, tt.home); got != tt.want {
			t.Errorf("contractHome(%q,%q) = %q, want %q", tt.path, tt.home, got, tt.want)
		}
	}
}

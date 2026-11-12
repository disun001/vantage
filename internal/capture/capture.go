// Package capture turns a live tmux session into a workspace definition.
//
// Capture never makes saved panes run anything by itself: a plain
// `ssh <alias>` becomes an ssh pane (the one thing restore runs on its own),
// and every other foreground command is saved with autorun = false.
package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// Lister reports the foreground command lines on a tty (see PS).
type Lister func(tty string) ([]string, error)

// Session reads the named session from the server and returns a workspace
// called name (defaults to the session name). procs may be nil, in which case
// non-shell panes keep only the command name.
func Session(c tmux.Client, session, name string, procs Lister) (workspace.Workspace, error) {
	if !c.HasSession(session) {
		return workspace.Workspace{}, fmt.Errorf("no tmux session %q", session)
	}
	panes, err := c.ListSessionPanes(session)
	if err != nil {
		return workspace.Workspace{}, err
	}
	if name == "" {
		name = session
	}
	home, _ := os.UserHomeDir()
	return build(name, home, panes, procs)
}

func build(name, home string, panes []tmux.PaneInfo, procs Lister) (workspace.Workspace, error) {
	ws := workspace.Workspace{Name: name}
	lastWindow := -1
	for _, p := range panes {
		if p.Window != lastWindow {
			ws.Windows = append(ws.Windows, workspace.Window{Name: p.WindowName, Layout: p.Layout})
			lastWindow = p.Window
		}
		w := &ws.Windows[len(ws.Windows)-1]
		pane := classify(p, procs)
		pane.CWD = contractHome(p.CWD, home)
		w.Panes = append(w.Panes, pane)
	}
	if err := ws.Validate(); err != nil {
		return ws, fmt.Errorf("captured workspace is invalid (try --name): %w", err)
	}
	return ws, nil
}

// shells are foreground commands that mean "idle prompt": nothing to save.
var shells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true,
	"ksh": true, "tcsh": true, "csh": true, "nu": true,
}

func classify(p tmux.PaneInfo, procs Lister) workspace.Pane {
	cmd := strings.TrimPrefix(p.Command, "-") // login shells show as "-zsh"
	if cmd == "" || shells[cmd] {
		return workspace.Pane{}
	}
	line := cmd
	if procs != nil {
		if found, ok := foreground(p, cmd, procs); ok {
			line = found
		}
	}
	if alias, ok := sshAlias(line); ok {
		return workspace.Pane{SSH: alias}
	}
	// Anything else is remembered but never run without the user opting in.
	return workspace.Pane{Cmd: line}
}

// foreground finds the full command line of the pane's foreground process.
func foreground(p tmux.PaneInfo, cmd string, procs Lister) (string, bool) {
	lines, err := procs(p.TTY)
	if err != nil {
		return "", false
	}
	for _, l := range lines {
		fields := strings.Fields(l)
		if len(fields) > 0 && filepath.Base(fields[0]) == cmd {
			return strings.Join(fields, " "), true
		}
	}
	return "", false
}

// sshAlias recognises exactly `ssh <alias>`. Anything with options, users or
// ports is not an alias and stays a plain (non-autorun) command, so capture
// never rewrites a connection into something different.
func sshAlias(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 || filepath.Base(fields[0]) != "ssh" {
		return "", false
	}
	if !workspace.ValidSSHAlias(fields[1]) {
		return "", false
	}
	return fields[1], true
}

func contractHome(path, home string) string {
	if home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

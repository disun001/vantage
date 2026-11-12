// Package tmux is a thin adapter over the tmux CLI.
//
// Safety rule: destructive server-level commands are only allowed when a
// dedicated socket is set, so a bug or a test can never kill the user's
// default tmux server.
package tmux

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Client runs tmux commands against one server.
type Client struct {
	// Socket is the `-L` socket name. Empty means the user's default server.
	Socket string
	// Bin is the tmux binary; defaults to "tmux".
	Bin string
}

func (c Client) run(args ...string) (string, error) {
	bin := c.Bin
	if bin == "" {
		bin = "tmux"
	}
	if c.Socket != "" {
		args = append([]string{"-L", c.Socket}, args...)
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	s := strings.TrimRight(string(out), "\n")
	if err != nil {
		return s, fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// HasSession reports whether a session with exactly this name exists.
func (c Client) HasSession(name string) bool {
	_, err := c.run("has-session", "-t", "="+name)
	return err == nil
}

// NewSession creates a detached session with one window and returns the
// first pane's ID. The size is generous so later splits don't fail with
// "no space for new pane" before a client attaches.
func (c Client) NewSession(name, window, cwd string) (string, error) {
	args := []string{"new-session", "-d", "-x", "240", "-y", "60", "-s", name, "-P", "-F", "#{pane_id}"}
	if window != "" {
		args = append(args, "-n", window)
	}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	return c.run(args...)
}

// NewWindow adds a window to an existing session and returns its first pane ID.
func (c Client) NewWindow(session, window, cwd string) (string, error) {
	args := []string{"new-window", "-t", "=" + session + ":", "-P", "-F", "#{pane_id}"}
	if window != "" {
		args = append(args, "-n", window)
	}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	return c.run(args...)
}

// SplitWindow splits the target pane. dir is "h" (side by side) or "v"
// (stacked). It returns the new pane's ID.
func (c Client) SplitWindow(pane, dir, cwd string) (string, error) {
	return c.split(pane, dirFlag(dir), cwd, "")
}

// SplitWindowRun is SplitWindow with a shell command run in the new pane
// instead of an interactive shell. The caller must have validated command.
func (c Client) SplitWindowRun(pane, dir, cwd, command string) (string, error) {
	return c.split(pane, dirFlag(dir), cwd, command)
}

func dirFlag(dir string) string {
	if dir == "v" {
		return "-v"
	}
	return "-h"
}

func (c Client) split(pane, flag, cwd, command string) (string, error) {
	args := []string{"split-window", flag, "-t", pane, "-P", "-F", "#{pane_id}"}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	if command != "" {
		args = append(args, command)
	}
	return c.run(args...)
}

// RespawnPane replaces whatever runs in pane with a shell command. The caller
// must have validated command.
func (c Client) RespawnPane(pane, command string) error {
	_, err := c.run("respawn-pane", "-k", "-t", pane, command)
	return err
}

// SelectLayout applies a layout preset or layout string to the pane's window.
func (c Client) SelectLayout(pane, layout string) error {
	_, err := c.run("select-layout", "-t", pane, layout)
	return err
}

// SendLine types text into a pane literally, then presses Enter.
func (c Client) SendLine(pane, text string) error {
	if _, err := c.run("send-keys", "-t", pane, "-l", text); err != nil {
		return err
	}
	_, err := c.run("send-keys", "-t", pane, "Enter")
	return err
}

// KillSession removes the session with exactly this name.
func (c Client) KillSession(name string) error {
	_, err := c.run("kill-session", "-t", "="+name)
	return err
}

// KillServer stops the whole server. Refused on the default server.
func (c Client) KillServer() error {
	if c.Socket == "" {
		return errors.New("refusing to kill the default tmux server")
	}
	_, err := c.run("kill-server")
	return err
}

// ListPanes returns `list-panes -a` output for the given format. Used by
// tests and, later, by capture and the sidebar.
func (c Client) ListPanes(format string) ([]string, error) {
	out, err := c.run("list-panes", "-a", "-F", format)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// PaneInfo describes one live pane, as needed by capture.
type PaneInfo struct {
	Window     int
	WindowName string
	Layout     string
	CWD        string
	// Command is the foreground command's name (tmux #{pane_current_command}).
	Command string
	// TTY is the pane's terminal device, e.g. /dev/ttys003.
	TTY string
}

// ListSessionPanes returns every pane of one session, ordered by window then
// pane index. WindowName is last in the format so a "|" in it cannot shift
// the other fields.
func (c Client) ListSessionPanes(session string) ([]PaneInfo, error) {
	const format = "#{window_index}|#{window_layout}|#{pane_current_path}|#{pane_current_command}|#{pane_tty}|#{window_name}"
	out, err := c.run("list-panes", "-s", "-t", "="+session, "-F", format)
	if err != nil {
		return nil, err
	}
	var panes []PaneInfo
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "|", 6)
		if len(f) != 6 {
			return nil, fmt.Errorf("unexpected list-panes line %q", line)
		}
		idx, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, fmt.Errorf("bad window index in %q: %w", line, err)
		}
		panes = append(panes, PaneInfo{Window: idx, Layout: f[1], CWD: f[2], Command: f[3], TTY: f[4], WindowName: f[5]})
	}
	return panes, nil
}

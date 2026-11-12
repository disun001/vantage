package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Session is one live session with its window count.
type Session struct {
	Name    string
	Windows int
}

// ListSessions returns the live sessions. A server that is not running has
// no sessions, so that is not an error.
func (c Client) ListSessions() ([]Session, error) {
	out, err := c.run("list-sessions", "-F", "#{session_windows}|#{session_name}")
	if err != nil {
		if strings.Contains(out, "no server running") || strings.Contains(out, "No such file") ||
			strings.Contains(out, "error connecting") {
			return nil, nil
		}
		return nil, err
	}
	var ss []Session
	for _, line := range strings.Split(out, "\n") {
		w, name, ok := strings.Cut(line, "|")
		n, convErr := strconv.Atoi(w)
		if !ok || convErr != nil {
			return nil, fmt.Errorf("unexpected list-sessions line %q", line)
		}
		ss = append(ss, Session{Name: name, Windows: n})
	}
	return ss, nil
}

// AttachCmd builds the command that attaches a client to a session. TMUX is
// removed so tmux does not refuse to nest when vantage itself runs in tmux,
// and TERM is fixed because the embedded emulator is xterm-compatible.
func (c Client) AttachCmd(session string) *exec.Cmd {
	bin := c.Bin
	if bin == "" {
		bin = "tmux"
	}
	args := []string{"attach-session", "-t", "=" + session}
	if c.Socket != "" {
		args = append([]string{"-L", c.Socket}, args...)
	}
	cmd := exec.Command(bin, args...)
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX=") && !strings.HasPrefix(kv, "TERM=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "TERM=xterm-256color")
	return cmd
}

// SwitchClient points the client on the given tty at a session.
func (c Client) SwitchClient(tty, session string) error {
	_, err := c.run("switch-client", "-c", tty, "-t", "="+session)
	return err
}

// PaneAtLeft reports whether the active pane of the client on tty touches
// the left edge of its window. It lets the TUI decide whether M-h moves to
// the sidebar or stays a tmux key.
func (c Client) PaneAtLeft(tty string) (bool, error) {
	out, err := c.run("display-message", "-p", "-c", tty, "#{pane_at_left}")
	if err != nil {
		return false, err
	}
	return out == "1", nil
}

// ClientTTY returns the tty of the attached client whose process id is pid.
// The embedded client's PTY slave name is not exposed by the PTY library, so
// tmux is asked instead. It fails until the client has registered.
func (c Client) ClientTTY(pid int) (string, error) {
	out, err := c.run("list-clients", "-F", "#{client_pid}|#{client_tty}")
	if err != nil {
		return "", err
	}
	want := strconv.Itoa(pid)
	for _, line := range strings.Split(out, "\n") {
		if p, tty, ok := strings.Cut(line, "|"); ok && p == want {
			return tty, nil
		}
	}
	return "", fmt.Errorf("no tmux client with pid %d", pid)
}

package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Session is one live session with its window count. ID is tmux's stable id
// ("$3"); window listings refer to sessions by it so a "|" in a name cannot
// shift fields.
type Session struct {
	ID      string
	Name    string
	Windows int
}

// ListSessions returns the live sessions. A server that is not running has
// no sessions, so that is not an error.
func (c Client) ListSessions() ([]Session, error) {
	out, err := c.run("list-sessions", "-F", "#{session_windows}|#{session_id}|#{session_name}")
	if err != nil {
		if strings.Contains(out, "no server running") || strings.Contains(out, "No such file") ||
			strings.Contains(out, "error connecting") {
			return nil, nil
		}
		return nil, err
	}
	var ss []Session
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "|", 3)
		if len(f) != 3 {
			return nil, fmt.Errorf("unexpected list-sessions line %q", line)
		}
		n, convErr := strconv.Atoi(f[0])
		if convErr != nil {
			return nil, fmt.Errorf("unexpected list-sessions line %q", line)
		}
		ss = append(ss, Session{ID: f[1], Name: f[2], Windows: n})
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

// ActivePane returns the ID and working directory of the active pane of the
// client on tty: the pane a split from that client's view should divide.
func (c Client) ActivePane(tty string) (id, cwd string, err error) {
	out, err := c.run("display-message", "-p", "-c", tty, "#{pane_id}|#{pane_current_path}")
	if err != nil {
		return "", "", err
	}
	id, cwd, _ = strings.Cut(out, "|")
	if !paneIDRe.MatchString(id) {
		return "", "", fmt.Errorf("unexpected pane id %q", id)
	}
	return id, cwd, nil
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

// Window is one window of a live session.
type Window struct {
	SessionID string
	Index     int
	Name      string
	Panes     int
	Active    bool // the session's current window
}

// ListWindows returns every window of every session, in tmux order. The
// window name is last in the format so a "|" in it cannot shift the fields.
func (c Client) ListWindows() ([]Window, error) {
	const format = "#{session_id}|#{window_index}|#{window_active}|#{window_panes}|#{window_name}"
	out, err := c.run("list-windows", "-a", "-F", format)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var ws []Window
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "|", 5)
		if len(f) != 5 {
			return nil, fmt.Errorf("unexpected list-windows line %q", line)
		}
		idx, err1 := strconv.Atoi(f[1])
		panes, err2 := strconv.Atoi(f[3])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("unexpected list-windows line %q", line)
		}
		ws = append(ws, Window{SessionID: f[0], Index: idx, Active: f[2] == "1", Panes: panes, Name: f[4]})
	}
	return ws, nil
}

// SelectWindow makes a window the current one of its session.
func (c Client) SelectWindow(session string, index int) error {
	_, err := c.run("select-window", "-t", "="+session+":"+strconv.Itoa(index))
	return err
}

// StatusBar reports how many lines the status bar of the client on tty takes
// and whether it sits at the top. Zero lines means it is off. The TUI uses it
// to crop the bar from the embedded view without touching the user's options.
func (c Client) StatusBar(tty string) (lines int, top bool, err error) {
	out, err := c.run("display-message", "-p", "-c", tty, "#{status}|#{status-position}")
	if err != nil {
		return 0, false, err
	}
	v, pos, _ := strings.Cut(out, "|")
	return statusLines(v), pos == "top", nil
}

// statusLines turns the value of the status option into a line count.
// Anything unknown counts as no bar, which crops nothing.
func statusLines(v string) int {
	switch v {
	case "on":
		return 1
	case "off", "":
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return 0
}

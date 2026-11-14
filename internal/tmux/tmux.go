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
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var paneIDRe = regexp.MustCompile(`^%[0-9]+$`)

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

// NewWindowRun is NewWindow with a shell command run in the window instead of
// an interactive shell. The caller must have validated command. The window
// becomes the session's current one.
func (c Client) NewWindowRun(session, window, command string) (string, error) {
	args := []string{"new-window", "-t", "=" + session + ":", "-P", "-F", "#{pane_id}"}
	if window != "" {
		args = append(args, "-n", window)
	}
	return c.run(append(args, command)...)
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

// RenamedOption is the pane option that records an ssh alias rename.
const RenamedOption = "@vantage_renamed"

var renamedTagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@%+-]*>[A-Za-z0-9][A-Za-z0-9._@%+-]*$`)

// SetRenamedTag marks the pane as running ssh to the alias from, which has
// since been renamed to the alias to. tag is "from>to".
func (c Client) SetRenamedTag(pane, tag string) error {
	if !paneIDRe.MatchString(pane) {
		return fmt.Errorf("invalid pane id %q", pane)
	}
	if !renamedTagRe.MatchString(tag) {
		return fmt.Errorf("invalid rename tag %q", tag)
	}
	_, err := c.run("set-option", "-p", "-t", pane, RenamedOption, tag)
	return err
}

// ClearRenamedTag removes the pane's rename tag. A pane that is gone is fine.
func (c Client) ClearRenamedTag(pane string) error {
	if !paneIDRe.MatchString(pane) {
		return fmt.Errorf("invalid pane id %q", pane)
	}
	out, err := c.run("set-option", "-p", "-u", "-t", pane, RenamedOption)
	if err != nil && strings.Contains(out, "can't find pane") {
		return nil
	}
	return err
}

// TitleOption is the pane option that holds a custom pane name. Empty or unset
// means the border follows the running command.
const TitleOption = "@vantage_name"

// SetPaneTitle names the pane. An empty title clears the name so the border
// goes back to the running command. The caller validates the title
// (workspace.ValidPaneTitle); the value only ever reaches tmux as one
// argument, never a shell.
func (c Client) SetPaneTitle(pane, title string) error {
	if !paneIDRe.MatchString(pane) {
		return fmt.Errorf("invalid pane id %q", pane)
	}
	if title == "" {
		out, err := c.run("set-option", "-p", "-u", "-t", pane, TitleOption)
		if err != nil && strings.Contains(out, "can't find pane") {
			return nil
		}
		return err
	}
	_, err := c.run("set-option", "-p", "-t", pane, TitleOption, title)
	return err
}

// PaneTitle returns the pane's custom name, empty when it has none.
func (c Client) PaneTitle(pane string) (string, error) {
	if !paneIDRe.MatchString(pane) {
		return "", fmt.Errorf("invalid pane id %q", pane)
	}
	return c.run("show-options", "-pqv", "-t", pane, TitleOption)
}

// PaneTitles returns the custom names of one session's panes, keyed by pane
// ID. Panes without a name are left out. The title is last in the format so a
// "|" in it cannot shift the pane ID.
func (c Client) PaneTitles(session string) (map[string]string, error) {
	out, err := c.run("list-panes", "-s", "-t", "="+session, "-F", "#{pane_id}|#{"+TitleOption+"}")
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		id, title, ok := strings.Cut(line, "|")
		if ok && title != "" {
			titles[id] = title
		}
	}
	return titles, nil
}

// IsNoServer reports whether err says no tmux server is running.
func IsNoServer(err error) bool {
	if err == nil {
		return false
	}
	m := err.Error()
	return strings.Contains(m, "no server running") || strings.Contains(m, "No such file") ||
		strings.Contains(m, "error connecting")
}

// KillPane closes one pane by id (%N). A pane that is already gone is not an
// error: the goal, no such pane, holds. Closing the last pane of a window
// closes the window, and the last window closes the session.
func (c Client) KillPane(pane string) error {
	if !paneIDRe.MatchString(pane) {
		return fmt.Errorf("%q is not a tmux pane id", pane)
	}
	if _, err := c.run("kill-pane", "-t", pane); err != nil && !strings.Contains(err.Error(), "can't find pane") {
		return err
	}
	return nil
}

// RenameSession gives the session called from a new name.
func (c Client) RenameSession(from, to string) error {
	_, err := c.run("rename-session", "-t", "="+from, to)
	return err
}

// RenameWindow gives window index of the session called session a new name.
func (c Client) RenameWindow(session string, index int, name string) error {
	_, err := c.run("rename-window", "-t", fmt.Sprintf("=%s:%d", session, index), name)
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
	// Pane is the pane ID, e.g. %4.
	Pane string
	// Title is the pane's custom name (see TitleOption), empty when it has
	// none. ListSessionPanes leaves it empty; capture fills it from PaneTitles.
	Title string
}

// ListSessionPanes returns every pane of one session, ordered by window then
// pane index. WindowName is last in the format so a "|" in it cannot shift
// the other fields.
func (c Client) ListSessionPanes(session string) ([]PaneInfo, error) {
	const format = "#{window_index}|#{window_layout}|#{pane_current_path}|#{pane_current_command}|#{pane_tty}|#{pane_id}|#{window_name}"
	out, err := c.run("list-panes", "-s", "-t", "="+session, "-F", format)
	if err != nil {
		return nil, err
	}
	var panes []PaneInfo
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "|", 7)
		if len(f) != 7 {
			return nil, fmt.Errorf("unexpected list-panes line %q", line)
		}
		idx, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, fmt.Errorf("bad window index in %q: %w", line, err)
		}
		panes = append(panes, PaneInfo{Window: idx, Layout: f[1], CWD: f[2], Command: f[3], TTY: f[4], Pane: f[5], WindowName: f[6]})
	}
	return panes, nil
}

// PaneRef locates one live pane anywhere on the server.
type PaneRef struct {
	Session    string
	Window     int
	WindowName string
	Pane       string // pane ID, e.g. %4
	TTY        string
	// Renamed is the pane's rename tag ("old>new", see SetRenamedTag), empty
	// when it has none.
	Renamed string
}

// ListAllPanes returns every pane of every session. WindowName is last in the
// format so a "|" in it cannot shift the other fields.
func (c Client) ListAllPanes() ([]PaneRef, error) {
	const format = "#{session_name}|#{window_index}|#{pane_id}|#{pane_tty}|#{" + RenamedOption + "}|#{window_name}"
	out, err := c.run("list-panes", "-a", "-F", format)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var refs []PaneRef
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "|", 6)
		if len(f) != 6 {
			return nil, fmt.Errorf("unexpected list-panes line %q", line)
		}
		idx, err := strconv.Atoi(f[1])
		if err != nil {
			return nil, fmt.Errorf("bad window index in %q: %w", line, err)
		}
		refs = append(refs, PaneRef{Session: f[0], Window: idx, Pane: f[2], TTY: f[3], Renamed: f[4], WindowName: f[5]})
	}
	return refs, nil
}

const bordersMarker = "@vantage_borders"

// paneBorderDefaults are the global window options ApplyPaneBorders sets, with
// the tmux default each one must still have for vantage to touch it. The
// value of each is built by borderValues, in the same order.
var paneBorderDefaults = []struct {
	opt  string
	defs []string
}{
	{"pane-border-lines", []string{"single"}},
	{"pane-border-style", []string{"default", "fg=themelightgrey"}},
	{"pane-active-border-style", []string{"fg=green", "fg=themegreen"}},
	{"pane-border-status", []string{"off"}},
	{"pane-border-format", []string{
		// tmux 3.8, then older releases without the mouse buttons.
		`#{?pane_active,#[reverse],}#{pane_index}#[default] "#{pane_title}"#{?#{mouse},#[align=right]#[range=control|7][#{?#{pane_floating_flag},t,f}]#[norange]#[range=control|8][#{?#{window_zoomed_flag},u,z}]#[norange]#[range=control|9][x]#[norange],}`,
		`#{?pane_active,#[reverse],}#{pane_index}#[default] "#{pane_title}"`,
	}},
}

// PaneBorderColors are the tmux color strings ApplyPaneBorders paints with.
type PaneBorderColors struct {
	Inactive string // inactive border line
	Active   string // active border line and active pane label
	Label    string // inactive pane label, brighter than its line
}

// borderValues are the values for paneBorderDefaults. The label is the pane's
// custom name or else the running command, like tmux windows. #{@vantage_name}
// is inserted as plain text, so a name is never expanded a second time.
func borderValues(col PaneBorderColors) []string {
	label := "#{?" + TitleOption + ",#{" + TitleOption + "},#{pane_current_command}}"
	format := "#{?pane_active,#[fg=" + col.Active + " bold],#[fg=" + col.Label + "]} " + label + " #[default]"
	return []string{"single", "fg=" + col.Inactive, "fg=" + col.Active, "top", format}
}

// ApplyPaneBorders gives panes a subtle border in the given colors and a title
// line on top of each pane. An option the user already changed from tmux's
// default is left alone. Errors are ignored: borders are cosmetic, and older
// tmux lacks pane-border-lines.
func (c Client) ApplyPaneBorders(col PaneBorderColors) {
	vals := borderValues(col)
	// A server vantage already styled keeps being restyled, so a color change
	// in vantage reaches it. Otherwise only tmux defaults are replaced.
	ours, _ := c.run("show-options", "-gqv", bordersMarker)
	set := false
	for i, o := range paneBorderDefaults {
		cur, err := c.run("show-options", "-gwv", o.opt)
		if err != nil || (ours == "" && !slices.Contains(o.defs, cur)) {
			continue
		}
		if _, err := c.run("set-option", "-gw", o.opt, vals[i]); err == nil {
			set = true
		}
	}
	if set {
		_, _ = c.run("set-option", "-g", bordersMarker, "1")
	}
}

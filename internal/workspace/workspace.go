// Package workspace defines the workspace definition file format
// (one TOML file per workspace), plus loading and validation.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// Workspace is one saved tmux session: windows, each with panes.
type Workspace struct {
	Name    string   `toml:"name"`
	Root    string   `toml:"root"`
	Windows []Window `toml:"window"`
}

// Window is one tmux window.
type Window struct {
	Name string `toml:"name"`
	// Layout is a tmux layout preset (e.g. "tiled") or a full layout string
	// as produced by `tmux list-windows -F '#{window_layout}'`.
	Layout string `toml:"layout"`
	Panes  []Pane `toml:"pane"`
}

// Pane is one tmux pane.
type Pane struct {
	CWD string `toml:"cwd"`
	// SSH is an alias from ~/.ssh/config. The pane runs `ssh <alias>`.
	SSH string `toml:"ssh"`
	// Cmd is a command to run in the pane's shell.
	Cmd string `toml:"cmd"`
	// Autorun must be true for Cmd to run on restore. Off by default so
	// nothing runs by surprise.
	Autorun bool `toml:"autorun"`
	// Split is how this pane is split from the previous one: "h" (side by
	// side) or "v" (stacked). Ignored for the first pane. Default "h".
	Split string `toml:"split"`
}

var (
	// tmux rewrites '.' and ':' in session names, and '=' / whitespace make
	// targets ambiguous, so keep names conservative.
	nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	// ssh aliases must not look like options, and must be one token.
	aliasRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@%+-]*$`)
)

// SSHCommand is the shell command a pane runs to connect to alias. The pane
// closes when ssh ends, like a local shell. Exit status 255 means ssh itself
// failed (unreachable host, auth), so it waits for Enter first or the error
// would vanish with the pane. alias must pass ValidSSHAlias.
func SSHCommand(alias string) string {
	return "ssh " + alias + `; rc=$?; if [ $rc -eq 255 ]; then echo; echo "ssh failed, press Enter"; read _; fi; exit $rc`
}

// ValidSSHAlias reports whether s is safe to pass to `ssh` as a host alias.
func ValidSSHAlias(s string) bool { return aliasRe.MatchString(s) }

// Load reads and validates a workspace file. If the file omits `name`, the
// file's base name (without extension) is used.
func Load(path string) (Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workspace{}, fmt.Errorf("%s: %w", path, err)
	}
	return parse(string(data), path)
}

// parse is Load for file text. path only names the file in errors and
// supplies the default workspace name.
func parse(text, path string) (Workspace, error) {
	var ws Workspace
	md, err := toml.Decode(text, &ws)
	if err != nil {
		return ws, fmt.Errorf("%s: %w", path, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		keys := make([]string, len(un))
		for i, k := range un {
			keys[i] = k.String()
		}
		return ws, fmt.Errorf("%s: unknown keys: %s", path, strings.Join(keys, ", "))
	}
	if ws.Name == "" {
		ws.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if err := ws.Validate(); err != nil {
		return ws, fmt.Errorf("%s: %w", path, err)
	}
	return ws, nil
}

// Validate checks structural rules.
func (w Workspace) Validate() error {
	if !nameRe.MatchString(w.Name) {
		return fmt.Errorf("invalid workspace name %q (use letters, digits, '-', '_')", w.Name)
	}
	if len(w.Windows) == 0 {
		return fmt.Errorf("workspace %q has no windows", w.Name)
	}
	for i, win := range w.Windows {
		if len(win.Panes) == 0 {
			return fmt.Errorf("window %d has no panes", i+1)
		}
		for j, p := range win.Panes {
			where := fmt.Sprintf("window %d pane %d", i+1, j+1)
			switch p.Split {
			case "", "h", "v":
			default:
				return fmt.Errorf("%s: split must be \"h\" or \"v\", got %q", where, p.Split)
			}
			if p.SSH != "" && p.Cmd != "" {
				return fmt.Errorf("%s: set either ssh or cmd, not both", where)
			}
			if p.SSH != "" && !aliasRe.MatchString(p.SSH) {
				return fmt.Errorf("%s: invalid ssh alias %q", where, p.SSH)
			}
			if p.Autorun && p.Cmd == "" {
				return fmt.Errorf("%s: autorun needs cmd", where)
			}
		}
	}
	return nil
}

// ResolveCWD returns the absolute directory for a pane, or "" to let tmux
// choose. Relative pane paths are joined to the workspace root; "~" expands
// to the home directory.
func (w Workspace) ResolveCWD(p Pane) string {
	root := expandHome(w.Root)
	dir := expandHome(p.CWD)
	switch {
	case dir == "":
		return root
	case filepath.IsAbs(dir) || root == "":
		return dir
	default:
		return filepath.Join(root, dir)
	}
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// Dir returns the workspaces directory: $VANTAGE_CONFIG_DIR/workspaces,
// else $XDG_CONFIG_HOME/vantage/workspaces, else ~/.config/vantage/workspaces.
// (os.UserConfigDir is avoided on purpose: on macOS it points at
// ~/Library/Application Support, which is not where dotfiles live.)
func Dir() (string, error) {
	if d := os.Getenv("VANTAGE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "workspaces"), nil
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "vantage", "workspaces"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "vantage", "workspaces"), nil
}

// Resolve turns a CLI argument into a file path: an existing path is used
// as is; otherwise it is looked up as <Dir>/<arg>.toml.
func Resolve(arg string) (string, error) {
	if strings.ContainsRune(arg, os.PathSeparator) || strings.HasSuffix(arg, ".toml") {
		return arg, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, arg+".toml"), nil
}

// List returns the workspace names (file base names) in Dir, sorted.
func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
		}
	}
	return names, nil
}

// UsingSSH returns the names of saved workspaces with a pane that connects to
// the ssh alias, sorted as List sorts them. Workspaces that cannot be read
// are skipped and reported together in the error, so one broken file never
// hides the others.
func UsingSSH(alias string) ([]string, error) {
	names, err := List()
	if err != nil {
		return nil, err
	}
	var used []string
	var bad []error
	for _, n := range names {
		path, err := PathOf(n)
		if err != nil {
			continue // not a name we can address
		}
		ws, err := Load(path)
		if err != nil {
			bad = append(bad, err)
			continue
		}
		if usesSSH(ws, alias) {
			used = append(used, n)
		}
	}
	return used, errors.Join(bad...)
}

func usesSSH(ws Workspace, alias string) bool {
	for _, w := range ws.Windows {
		for _, p := range w.Panes {
			if p.SSH == alias {
				return true
			}
		}
	}
	return false
}

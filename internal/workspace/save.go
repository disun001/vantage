package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// savePane mirrors Pane for writing. Fields are omitted when empty, and
// Autorun is a pointer so a command pane can state `autorun = false`
// explicitly while other panes stay terse.
type savePane struct {
	CWD     string `toml:"cwd,omitempty"`
	SSH     string `toml:"ssh,omitempty"`
	Cmd     string `toml:"cmd,omitempty"`
	Chat    string `toml:"chat,omitempty"`
	Autorun *bool  `toml:"autorun,omitempty"`
	Split   string `toml:"split,omitempty"`
}

type saveWindow struct {
	Name   string     `toml:"name"`
	Layout string     `toml:"layout,omitempty"`
	Panes  []savePane `toml:"pane"`
}

type saveWorkspace struct {
	Name    string       `toml:"name"`
	Root    string       `toml:"root,omitempty"`
	Project string       `toml:"project,omitempty"`
	Task    string       `toml:"task,omitempty"`
	Windows []saveWindow `toml:"window"`
}

// Save validates ws and writes it to path, creating parent directories. An
// existing file is only replaced when overwrite is true.
func Save(path string, ws Workspace, overwrite bool) (err error) {
	if err := ws.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		ws = keepLink(path, ws)
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s already exists (use --force to replace it)", path)
	}
	if err != nil {
		return err
	}
	// A failed Close can mean lost data, so it is part of the result.
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return toml.NewEncoder(f).Encode(toSave(ws))
}

func toSave(ws Workspace) saveWorkspace {
	out := saveWorkspace{Name: ws.Name, Root: ws.Root, Project: ws.Project, Task: ws.Task}
	for _, w := range ws.Windows {
		sw := saveWindow{Name: w.Name, Layout: w.Layout}
		for _, p := range w.Panes {
			sp := savePane{CWD: p.CWD, SSH: p.SSH, Cmd: p.Cmd, Chat: p.Chat, Split: p.Split}
			if p.Cmd != "" {
				a := p.Autorun
				sp.Autorun = &a
			}
			sw.Panes = append(sw.Panes, sp)
		}
		out.Windows = append(out.Windows, sw)
	}
	return out
}

// SaveBackup is Save with overwrite, for replacing a file the user may have
// edited: the old file is first copied to path + BackupSuffix, so comments and
// opted-in `autorun` lines can be recovered. A missing file needs no backup.
func SaveBackup(path string, ws Workspace) error {
	if err := ws.Validate(); err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := os.WriteFile(path+BackupSuffix, old, 0o600); err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	return Save(path, ws, true)
}

// keepLink carries the project and task of the file being replaced into ws
// when ws has none. A capture reads a live session, which knows nothing of
// projects, and must not sever a task workspace from its task.
func keepLink(path string, ws Workspace) Workspace {
	if ws.Project != "" {
		return ws
	}
	old, err := Load(path)
	if err != nil {
		return ws
	}
	ws.Project, ws.Task = old.Project, old.Task
	return ws
}

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
	out := saveWorkspace{Name: ws.Name, Root: ws.Root}
	for _, w := range ws.Windows {
		sw := saveWindow{Name: w.Name, Layout: w.Layout}
		for _, p := range w.Panes {
			sp := savePane{CWD: p.CWD, SSH: p.SSH, Cmd: p.Cmd, Split: p.Split}
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

// Package manage creates, renames and deletes workspaces. A workspace is a
// saved TOML file; the tmux session of the same name, when it is running, is
// kept in step.
package manage

import (
	"fmt"
	"os"

	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// Manager works on one tmux server and the workspaces directory.
type Manager struct {
	Client tmux.Client
	// Procs finds foreground commands for capture. May be nil.
	Procs capture.Lister
}

func checkName(name string) error {
	if !workspace.ValidName(name) {
		return fmt.Errorf("invalid name %q (letters, digits, '-', '_')", name)
	}
	return nil
}

// free fails when a workspace file or a live session already has the name.
func (m Manager) free(name string) error {
	if saved, err := workspace.Exists(name); err != nil {
		return err
	} else if saved {
		return fmt.Errorf("workspace %q already exists", name)
	}
	if m.Client.HasSession(name) {
		return fmt.Errorf("session %q already exists", name)
	}
	return nil
}

// Create starts a session called name in the home directory and saves it as
// a workspace. If saving fails the new session is removed again.
func (m Manager) Create(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := m.free(name); err != nil {
		return err
	}
	path, err := workspace.PathOf(name)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	if _, err := m.Client.NewSession(name, "", home); err != nil {
		return err
	}
	ws, err := capture.Session(m.Client, name, "", m.Procs)
	if err == nil {
		err = workspace.Save(path, ws, false)
	}
	if err != nil {
		_ = m.Client.KillSession(name)
		return err
	}
	return nil
}

// Rename renames the saved workspace and the live session, whichever exist.
// If the file cannot be renamed, the session name is put back.
func (m Manager) Rename(from, to string) error {
	if err := checkName(to); err != nil {
		return err
	}
	if from == to {
		return nil
	}
	saved, err := workspace.Exists(from)
	if err != nil {
		return err
	}
	live := m.Client.HasSession(from)
	if !saved && !live {
		return fmt.Errorf("%q is neither saved nor running", from)
	}
	if err := m.free(to); err != nil {
		return err
	}
	if live {
		if err := m.Client.RenameSession(from, to); err != nil {
			return err
		}
	}
	if saved {
		if err := workspace.Rename(from, to); err != nil {
			if live {
				_ = m.Client.RenameSession(to, from)
			}
			return err
		}
	}
	return nil
}

// Delete removes the saved workspace file and kills the live session,
// whichever exist.
func (m Manager) Delete(name string) error {
	saved, err := workspace.Exists(name)
	if err != nil {
		return err
	}
	live := m.Client.HasSession(name)
	if !saved && !live {
		return fmt.Errorf("%q is neither saved nor running", name)
	}
	if saved {
		if err := workspace.Delete(name); err != nil {
			return err
		}
	}
	if live {
		return m.Client.KillSession(name)
	}
	return nil
}

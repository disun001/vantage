package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/hostuse"
	"github.com/disun001/vantage/internal/manage"
	"github.com/disun001/vantage/internal/restore"
	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/tui"
	"github.com/disun001/vantage/internal/workspace"
)

var tuiCmd = command{
	name:    "tui",
	usage:   "tui",
	summary: "sidebar + embedded tmux (--socket NAME)",
	run:     runTUI,
}

func runTUI(args []string, _ io.Writer) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	socket := fs.String("socket", os.Getenv("VANTAGE_SOCKET"), "tmux socket name (-L)")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home directory: %w", err)
	}
	keyLog, closeLog, err := openKeyLog(os.Getenv("VANTAGE_KEYLOG"))
	if err != nil {
		return err
	}
	defer closeLog()
	client := tmux.Client{Socket: *socket}
	return tui.Run(tui.Config{
		Client:  client,
		Saved:   workspace.List,
		Manage:  manage.Manager{Client: client, Procs: capture.PS},
		SSH:     sshconfig.File{Path: filepath.Join(home, ".ssh", "config")},
		HostUse: hostuse.Finder{Client: client, Procs: capture.PS},
		KeyLog:  keyLog,
		Restore: func(name string) error {
			path, err := workspace.Resolve(name)
			if err != nil {
				return err
			}
			ws, err := workspace.Load(path)
			if err != nil {
				return err
			}
			_, err = restore.Restore(client, ws)
			return err
		},
	})
}

// openKeyLog opens the file named by VANTAGE_KEYLOG for appending. An empty
// path means no log. The log holds key names, never typed text.
func openKeyLog(path string) (io.Writer, func(), error) {
	if path == "" {
		return nil, func() {}, nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("key log: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/disun001/vantage/internal/restore"
	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

var restoreCmd = command{
	name:    "restore",
	usage:   "restore <name|path>",
	summary: "create the tmux session for a workspace (--socket NAME)",
	run:     runRestore,
}

func runRestore(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	socket := fs.String("socket", os.Getenv("VANTAGE_SOCKET"), "tmux socket name (-L)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("restore needs exactly one workspace name or path")
	}

	path, err := workspace.Resolve(pos[0])
	if err != nil {
		return err
	}
	ws, err := loadWorkspace(path)
	if err != nil {
		return err
	}
	res, err := restore.Restore(tmux.Client{Socket: *socket}, ws)
	if err != nil {
		return err
	}
	if res.Existed {
		fmt.Fprintf(out, "%s: already running\n", ws.Name)
		return nil
	}
	fmt.Fprintf(out, "%s: restored (%d windows, %d panes)\n", ws.Name, res.Windows, res.Panes)
	return nil
}

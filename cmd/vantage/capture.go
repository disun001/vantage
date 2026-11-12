package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

var captureCmd = command{
	name:    "capture",
	usage:   "capture <session>",
	summary: "save a running session as a workspace (--name, --force, --socket)",
	run:     runCapture,
}

func runCapture(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	socket := fs.String("socket", os.Getenv("VANTAGE_SOCKET"), "tmux socket name (-L)")
	name := fs.String("name", "", "workspace name (default: the session name)")
	force := fs.Bool("force", false, "replace an existing workspace file")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("capture needs exactly one session name")
	}

	ws, err := capture.Session(tmux.Client{Socket: *socket}, pos[0], *name, capture.PS)
	if err != nil {
		return err
	}
	path, err := workspace.Resolve(ws.Name)
	if err != nil {
		return err
	}
	if err := workspace.Save(path, ws, *force); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s: saved to %s\n", ws.Name, path)
	return nil
}

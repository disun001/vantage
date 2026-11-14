package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

var renamePaneCmd = command{
	name:    "rename-pane",
	usage:   "rename-pane [title]",
	summary: "name a pane (empty title: follow the running command); asks when no title is given",
	run:     runRenamePane,
}

func runRenamePane(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("rename-pane", flag.ContinueOnError)
	socket := fs.String("socket", os.Getenv("VANTAGE_SOCKET"), "tmux socket name (-L)")
	pane := fs.String("pane", os.Getenv("TMUX_PANE"), "pane to rename (default: $TMUX_PANE)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return errors.New("rename-pane takes one title: quote it if it has spaces")
	}
	if !paneRe.MatchString(*pane) {
		return fmt.Errorf("pane %q is not a tmux pane id (run inside tmux or pass --pane %%N)", *pane)
	}
	return renamePane(tmux.Client{Socket: *socket}, *pane, pos, os.Stdin, out)
}

// renamePane sets the pane's name from args, or from one line read from in
// when args is empty. The title never passes through a shell: the tmux
// binding opens a popup that runs this command, and the name is typed here.
func renamePane(c tmux.Client, pane string, args []string, in io.Reader, out io.Writer) error {
	var title string
	if len(args) == 1 {
		title = args[0]
	} else {
		cur, err := c.PaneTitle(pane)
		if err != nil {
			return err
		}
		if cur != "" {
			fmt.Fprintf(out, "current: %s\n", cur)
		}
		fmt.Fprint(out, "pane name (empty = follow the command): ")
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			return errors.New("cancelled")
		}
		title = strings.TrimRight(line, "\r\n")
	}
	title = strings.TrimSpace(title)
	if err := workspace.ValidPaneTitle(title); err != nil {
		return err
	}
	return c.SetPaneTitle(pane, title)
}

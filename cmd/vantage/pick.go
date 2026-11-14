package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/connections"
	"github.com/disun001/vantage/internal/pick"
	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/tmux"
)

var pickCmd = command{
	name:    "pick",
	usage:   "pick --split h|v",
	summary: "split the pane as local or ssh, chosen from a menu (for tmux popups)",
	run:     runPick,
}

// paneRe is a tmux pane ID. Anything else could be read by tmux as a flag or
// another kind of target.
var paneRe = regexp.MustCompile(`^%[0-9]+$`)

func runPick(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pick", flag.ContinueOnError)
	socket := fs.String("socket", os.Getenv("VANTAGE_SOCKET"), "tmux socket name (-L)")
	split := fs.String("split", "", "split direction: h (side by side) or v (stacked)")
	pane := fs.String("pane", os.Getenv("TMUX_PANE"), "pane to split (default: $TMUX_PANE)")
	cwd := fs.String("cwd", "", "working directory for the new pane")
	sshPath := fs.String("ssh-config", "", "ssh config path (default: ~/.ssh/config)")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	if *split != "h" && *split != "v" {
		return errors.New("pick needs --split h or --split v")
	}
	if !paneRe.MatchString(*pane) {
		return fmt.Errorf("pane %q is not a tmux pane id (run inside tmux or pass --pane %%N)", *pane)
	}

	if *sshPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home directory: %w", err)
		}
		*sshPath = filepath.Join(home, ".ssh", "config")
	}
	cfgHosts, err := sshconfig.Load(*sshPath)
	if err != nil {
		return err
	}
	client := tmux.Client{Socket: *socket}
	// The open-connection indicator is a nicety: if tmux or ps cannot answer,
	// show the menu without it instead of failing the split.
	open, _ := connections.Find(client, capture.PS)
	hosts := make([]pick.Host, len(cfgHosts))
	for i, h := range cfgHosts {
		hosts[i] = pick.Host{
			Alias: h.Alias, HostName: h.HostName, User: h.User, Port: h.Port,
			ProxyJump: h.ProxyJump, Open: connections.Lookup(open, h.Alias, h.HostName),
		}
	}

	choice, err := pick.Choose(os.Stdin, out, pick.Context{Dir: *split, CWD: *cwd}, hosts)
	if errors.Is(err, pick.ErrCancelled) {
		return nil // quitting the menu is not an error
	}
	if err != nil {
		return err
	}
	pw, err := newPasswords(*sshPath)
	if err != nil {
		return err
	}
	choice.Askpass = pw.askpassFor(nil)(choice.Alias)
	_, err = pick.Split(client, *pane, *split, *cwd, choice)
	return err
}

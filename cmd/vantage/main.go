// Command vantage manages tmux workspaces.
//
// main.go only dispatches. Each subcommand lives in its own file and
// registers itself in commands below.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// command is one subcommand. run receives the arguments after the
// subcommand name and writes normal output to out.
type command struct {
	name    string
	usage   string // e.g. "restore <name|path>"
	summary string
	run     func(args []string, out io.Writer) error
}

// commands is the single place subcommands are registered.
var commands = []command{listCmd, restoreCmd, captureCmd, pickCmd}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without os.Exit so it can be tested. It returns the exit code.
func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, usageText())
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(out, usageText())
		return 0
	}
	for _, c := range commands {
		if c.name == args[0] {
			if err := c.run(args[1:], out); err != nil {
				fmt.Fprintln(errOut, "vantage:", err)
				return 1
			}
			return 0
		}
	}
	fmt.Fprintf(errOut, "unknown command %q\n\n%s", args[0], usageText())
	return 2
}

func usageText() string {
	var b strings.Builder
	b.WriteString("vantage - tmux workspaces\n\nusage:\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  vantage %-24s %s\n", c.usage, c.summary)
	}
	return b.String()
}

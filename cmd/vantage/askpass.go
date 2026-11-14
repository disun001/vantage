package main

import (
	"fmt"
	"io"
	"os"

	"github.com/disun001/vantage/internal/askpass"
)

// runAskpass is what ssh runs as SSH_ASKPASS: vantage started with the
// alias in its environment and ssh's prompt as the only argument. It is not a
// listed subcommand; people never call it.
func runAskpass(args []string, out, errOut io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(errOut, "vantage: askpass is run by ssh with a prompt")
		return 2
	}
	tty, err := askpass.OpenTTY()
	if err != nil {
		fmt.Fprintln(errOut, "vantage:", err)
		return 1
	}
	defer tty.Close()
	return askpass.Run(os.Getenv, args[0], out, tty)
}

package askpass

import (
	"fmt"
	"io"
	"strings"
)

// Terminal is where the helper asks the person when the vault cannot answer.
// It is the controlling terminal of the ssh pane, not stdin or stdout, which
// ssh uses to talk to the helper.
type Terminal interface {
	// Show prints text without a newline.
	Show(text string)
	// ReadLine reads an echoed line.
	ReadLine() (string, error)
	// ReadSecret reads a line without echo.
	ReadSecret() (string, error)
}

// Run is what ssh executes: it answers prompt on out (the password and a
// newline) and returns the exit status. env reads environment variables. term is
// used for everything the vault does not answer, so ssh behaves as it would
// without the helper.
func Run(env func(string) string, prompt string, out io.Writer, term Terminal) int {
	alias, sock := env(EnvAlias), env(EnvSocket)
	if alias != "" && sock != "" && env(EnvPrompt) == "" {
		if pw, ok, err := Ask(sock, alias, prompt); err == nil && ok {
			_, err := fmt.Fprintln(out, pw)
			return exit(err == nil)
		}
	}
	switch env(EnvPrompt) {
	case "none": // information only
		term.Show(prompt + "\n")
		return 0
	case "confirm":
		term.Show(prompt + " [y/N] ")
		a, err := term.ReadLine()
		return exit(err == nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(a)), "y"))
	}
	term.Show(prompt)
	a, err := term.ReadSecret()
	term.Show("\n")
	if err != nil {
		return 1
	}
	_, err = fmt.Fprintln(out, a)
	return exit(err == nil)
}

func exit(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

// Package pick chooses where a new split runs (the local shell or an ssh host)
// and creates it. It reads choices from an io.Reader so it works in a tmux
// popup and in tests, and never touches ~/.ssh itself.
package pick

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// Choice is where the new pane runs. An empty Alias means the local shell.
type Choice struct {
	Alias string
}

// Choose prints a numbered menu to out and reads one line from in. Empty
// input, "0", or "l" picks local; a number picks that host; an exact alias
// also works. Anything else is an error so a typo never opens a wrong host.
func Choose(in io.Reader, out io.Writer, hosts []string) (Choice, error) {
	fmt.Fprintln(out, "  0) local shell (default)")
	for i, h := range hosts {
		fmt.Fprintf(out, "%3d) ssh %s\n", i+1, h)
	}
	fmt.Fprint(out, "choose: ")

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return Choice{}, fmt.Errorf("read choice: %w", err)
	}
	return parse(strings.TrimSpace(line), hosts)
}

func parse(s string, hosts []string) (Choice, error) {
	if s == "" || s == "0" || s == "l" {
		return Choice{}, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n < 1 || n > len(hosts) {
			return Choice{}, fmt.Errorf("no choice %d", n)
		}
		return Choice{Alias: hosts[n-1]}, nil
	}
	for _, h := range hosts {
		if h == s {
			return Choice{Alias: h}, nil
		}
	}
	return Choice{}, fmt.Errorf("unknown choice %q", s)
}

// Split splits pane in direction dir ("h" side by side, "v" stacked). For an
// ssh choice the new pane runs ssh directly, so nothing is typed or echoed and
// it feels like opening on the remote host; when ssh ends it falls back to a
// local shell instead of closing the pane. It returns the new pane's ID.
func Split(c tmux.Client, pane, dir, cwd string, ch Choice) (string, error) {
	if dir != "h" && dir != "v" {
		return "", fmt.Errorf("split direction must be h or v, got %q", dir)
	}
	if ch.Alias == "" {
		id, err := c.SplitWindow(pane, dir, cwd)
		if err != nil {
			return "", fmt.Errorf("split: %w", err)
		}
		return id, nil
	}
	if !workspace.ValidSSHAlias(ch.Alias) {
		return "", fmt.Errorf("invalid ssh alias %q", ch.Alias)
	}
	// The alias is validated above, so it is safe inside this shell command.
	// ${SHELL:-sh} is expanded by the pane's shell, not by us.
	id, err := c.SplitWindowRun(pane, dir, cwd, "ssh "+ch.Alias+`; exec "${SHELL:-sh}"`)
	if err != nil {
		return "", fmt.Errorf("split: %w", err)
	}
	return id, nil
}

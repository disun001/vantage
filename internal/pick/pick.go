// Package pick chooses where a new split runs (the local shell or an ssh host)
// with a small fuzzy menu, and creates the split. It never reads ~/.ssh itself:
// the caller passes the host aliases in.
package pick

import (
	"fmt"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

// Choice is where the new pane runs. An empty Alias means the local shell.
type Choice struct {
	Alias string
	// Askpass, when set, lets the new pane answer the host's password prompt
	// from the vault.
	Askpass *workspace.Askpass
}

// Split splits pane in direction dir ("h" side by side, "v" stacked). For an
// ssh choice the new pane runs ssh directly, so nothing is typed or echoed and
// it feels like opening on the remote host, and the pane closes when ssh ends. It returns the new pane's ID.
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
	id, err := c.SplitWindowRun(pane, dir, cwd, workspace.SSHCommandWith(ch.Alias, ch.Askpass))
	if err != nil {
		return "", fmt.Errorf("split: %w", err)
	}
	return id, nil
}

// Package git is a thin adapter over the git CLI: the only place that execs
// git. It never rewrites history or discards work; callers decide policy.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Client runs git. The zero value uses `git` from PATH.
type Client struct {
	// Bin overrides the git binary (tests).
	Bin string
}

// Status is the state of one checkout.
type Status struct {
	// Branch is the checked-out branch, or "" when HEAD is detached.
	Branch string
	// Default is the remote's default branch (origin/HEAD), or "" if unknown.
	Default string
	// HasUpstream is false when Branch tracks nothing; Ahead and Behind are
	// then 0.
	HasUpstream   bool
	Ahead, Behind int
	// Dirty means uncommitted changes or untracked files.
	Dirty bool
	// Worktrees counts linked worktrees, not the main one.
	Worktrees int
}

func (c Client) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "git"
}

// run executes git in dir (or the current directory when empty). Prompts are
// off so a missing credential fails instead of hanging the caller.
func (c Client) run(dir string, args ...string) (string, error) {
	cmd := exec.Command(c.bin(), args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("git %s: %w", args[0], err)
		}
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Clone clones url into dest. url must already be validated: it follows
// `--` so it cannot be read as an option, and the ext transport (which runs
// commands) is disabled.
func (c Client) Clone(url, dest string) error {
	_, err := c.run("", "-c", "protocol.ext.allow=never", "clone", "--", url, dest)
	return err
}

// Fetch updates remote-tracking branches of the checkout in dir.
func (c Client) Fetch(dir string) error {
	_, err := c.run(dir, "fetch", "--prune")
	return err
}

// Status reports the state of the checkout in dir.
func (c Client) Status(dir string) (Status, error) {
	var st Status
	br, err := c.run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return st, err
	}
	if br != "HEAD" {
		st.Branch = br
	}
	if ref, err := c.run(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		st.Default = strings.TrimPrefix(ref, "origin/")
	}
	if out, err := c.run(dir, "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		if f := strings.Fields(out); len(f) == 2 {
			st.HasUpstream = true
			st.Ahead, _ = strconv.Atoi(f[0])
			st.Behind, _ = strconv.Atoi(f[1])
		}
	}
	porcelain, err := c.run(dir, "status", "--porcelain")
	if err != nil {
		return st, err
	}
	st.Dirty = porcelain != ""
	wt, err := c.run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return st, err
	}
	st.Worktrees = strings.Count("\n"+wt, "\nworktree ") - 1
	return st, nil
}

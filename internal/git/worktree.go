package git

import (
	"fmt"
	"strconv"
	"strings"
)

// RefExists reports whether ref (a full ref name such as refs/heads/x)
// resolves to a commit.
func (c Client) RefExists(dir, ref string) bool {
	_, err := c.run(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// WorktreeAdd creates a linked worktree at path on a new branch cut from
// start. The branch does not track start, so a later push is a deliberate
// choice. branch and start must already be validated.
func (c Client) WorktreeAdd(dir, path, branch, start string) error {
	_, err := c.run(dir, "worktree", "add", "--no-track", "-b", branch, "--", path, start)
	return err
}

// WorktreeRemove removes the linked worktree at path. Without force git
// refuses when the worktree has uncommitted or untracked files.
func (c Client) WorktreeRemove(dir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	_, err := c.run(dir, append(args, "--", path)...)
	return err
}

// DeleteBranch deletes branch only if it is fully merged (`git branch -d`).
// It never uses -D: unmerged work is kept.
func (c Client) DeleteBranch(dir, branch string) error {
	_, err := c.run(dir, "branch", "-d", "--", branch)
	return err
}

// Unmerged counts commits on branch that base does not have.
func (c Client) Unmerged(dir, branch, base string) (int, error) {
	return c.count(dir, base+".."+branch)
}

// Unpushed counts commits on branch that no remote-tracking branch has.
func (c Client) Unpushed(dir, branch string) (int, error) {
	return c.count(dir, branch, "--not", "--remotes")
}

func (c Client) count(dir string, revs ...string) (int, error) {
	out, err := c.run(dir, append([]string{"rev-list", "--count"}, revs...)...)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("rev-list count %q: %w", out, err)
	}
	return n, nil
}

// IsTracked reports whether the repo-relative path rel is tracked by git.
func (c Client) IsTracked(dir, rel string) bool {
	_, err := c.run(dir, "ls-files", "--error-unmatch", "--", rel)
	return err == nil
}

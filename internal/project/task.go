package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TaskRepo selects one repo for a task and the branch to cut it from.
type TaskRepo struct {
	// Dir is the repo's DirName.
	Dir string
	// Base is the branch to start from; empty means the repo's default.
	Base string
}

var branchRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// ValidTaskName checks a task name. The name is also the branch name and a
// directory name, so it uses the project-name charset.
func ValidTaskName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid task name %q (use letters, digits, '-', '_')", name)
	}
	return nil
}

// validBranch accepts the common shapes of a branch name and rejects anything
// that could be read as an option or a revision expression.
func validBranch(b string) error {
	bad := !branchRe.MatchString(b) || strings.Contains(b, "..") || strings.Contains(b, "//") ||
		strings.Contains(b, "/.") || strings.HasSuffix(b, "/") || strings.HasSuffix(b, ".") ||
		strings.HasSuffix(b, ".lock")
	if bad {
		return fmt.Errorf("invalid branch name %q", b)
	}
	return nil
}

// TasksPath is where the task dirs of p live under root.
func (p Project) TasksPath(root string) string { return filepath.Join(root, ".tasks", p.Name) }

// TaskPath is the root of one task.
func (p Project) TaskPath(root, task string) string { return filepath.Join(p.TasksPath(root), task) }

// TaskRepoPath is the worktree of r for a task. A flat project's worktree is
// the task root itself.
func (p Project) TaskRepoPath(root, task string, r Repo) string {
	if p.Flat {
		return p.TaskPath(root, task)
	}
	return filepath.Join(p.TaskPath(root, task), r.DirName())
}

// ListTasks returns the task names of p, sorted.
func ListTasks(root string, p Project) ([]string, error) {
	entries, err := os.ReadDir(p.TasksPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && ValidTaskName(e.Name()) == nil {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// TaskRepos returns the repos of p that have a worktree in the task.
func TaskRepos(root string, p Project, task string) []Repo {
	var out []Repo
	for _, r := range p.Repos {
		if _, err := os.Stat(filepath.Join(p.TaskRepoPath(root, task, r), ".git")); err == nil {
			out = append(out, r)
		}
	}
	return out
}

// findRepo returns the repo of p whose DirName is dir.
func (p Project) findRepo(dir string) (Repo, error) {
	names := make([]string, len(p.Repos))
	for i, r := range p.Repos {
		if r.DirName() == dir {
			return r, nil
		}
		names[i] = r.DirName()
	}
	return Repo{}, fmt.Errorf("unknown repo %q (have: %s)", dir, strings.Join(names, ", "))
}

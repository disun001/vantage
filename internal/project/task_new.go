package project

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/disun001/vantage/internal/git"
)

// planned is one worktree NewTask is about to create.
type planned struct {
	repo  Repo
	main  string // main clone
	path  string // worktree
	start string // start point for the new branch
}

// NewTask creates a task: for each selected repo (all of them when repos is
// empty) a worktree on a new branch named task, cut from the fetched base.
// It is all or nothing: every check runs before the first worktree is made,
// and a failure part way removes what was made. Then the repo's copy files
// are copied in; a copy failure is reported but keeps the task.
func NewTask(root string, p Project, g git.Client, task string, repos []TaskRepo, out io.Writer) error {
	if err := ValidTaskName(task); err != nil {
		return err
	}
	plan, err := planTask(root, p, g, task, repos)
	if err != nil {
		return err
	}
	if err := createWorktrees(root, p, g, task, plan, out); err != nil {
		return err
	}
	var errs []error
	for _, w := range plan {
		if err := copyFiles(g, w.repo.DirName(), w.main, w.path, w.repo.Copy, out); err != nil {
			errs = append(errs, err)
		}
	}
	fmt.Fprintf(out, "task    %s\n", p.TaskPath(root, task))
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("task created, but copying files failed: %w", err)
	}
	return nil
}

// planTask runs every check that can fail before anything is created.
func planTask(root string, p Project, g git.Client, task string, repos []TaskRepo) ([]planned, error) {
	if _, err := os.Lstat(p.TaskPath(root, task)); err == nil {
		return nil, fmt.Errorf("task %q already exists", task)
	}
	if len(repos) == 0 {
		for _, r := range p.Repos {
			repos = append(repos, TaskRepo{Dir: r.DirName()})
		}
	}
	if p.Flat && len(repos) != 1 {
		return nil, errors.New("a flat project has exactly one repo")
	}
	var plan []planned
	seen := map[string]bool{}
	for _, tr := range repos {
		r, err := p.findRepo(tr.Dir)
		if err != nil {
			return nil, err
		}
		if seen[tr.Dir] {
			return nil, fmt.Errorf("repo %q selected twice", tr.Dir)
		}
		seen[tr.Dir] = true
		w, err := planRepo(root, p, g, task, r, tr.Base)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.DirName(), err)
		}
		plan = append(plan, w)
	}
	return plan, nil
}

func planRepo(root string, p Project, g git.Client, task string, r Repo, base string) (planned, error) {
	w := planned{repo: r, main: p.RepoPath(root, r), path: p.TaskRepoPath(root, task, r)}
	if cloned, err := isClone(w.main); err != nil {
		return w, err
	} else if !cloned {
		return w, errors.New("not cloned (run: vantage project sync)")
	}
	if err := g.Fetch(w.main); err != nil {
		return w, err
	}
	if g.RefExists(w.main, "refs/heads/"+task) {
		return w, fmt.Errorf("branch %q already exists", task)
	}
	start, err := startPoint(g, w.main, base)
	w.start = start
	return w, err
}

// startPoint resolves base (or the repo's default) to the fetched remote
// branch, falling back to a local branch of that name.
func startPoint(g git.Client, dir, base string) (string, error) {
	if base == "" {
		st, err := g.Status(dir)
		if err != nil {
			return "", err
		}
		if st.Default == "" {
			return "", errors.New("default branch unknown; name a base as repo:branch")
		}
		base = st.Default
	}
	if err := validBranch(base); err != nil {
		return "", err
	}
	switch {
	case g.RefExists(dir, "refs/remotes/origin/"+base):
		return "origin/" + base, nil
	case g.RefExists(dir, "refs/heads/"+base):
		return base, nil
	}
	return "", fmt.Errorf("base branch %q not found", base)
}

func createWorktrees(root string, p Project, g git.Client, task string, plan []planned, out io.Writer) error {
	taskDir := p.TaskPath(root, task)
	mk := taskDir
	if p.Flat {
		mk = p.TasksPath(root) // git creates the worktree dir itself
	}
	if err := os.MkdirAll(mk, 0o755); err != nil {
		return err
	}
	var made []planned
	for _, w := range plan {
		if err := g.WorktreeAdd(w.main, w.path, task, w.start); err != nil {
			rollback(g, task, made, taskDir, out)
			return fmt.Errorf("%s: %w", w.repo.DirName(), err)
		}
		made = append(made, w)
		fmt.Fprintf(out, "created %s (from %s)\n", w.repo.DirName(), w.start)
	}
	return nil
}

// rollback undoes the worktrees made so far. The branches were just created
// with no commits, so the safe `branch -d` is enough; a leftover is reported.
func rollback(g git.Client, task string, made []planned, taskDir string, out io.Writer) {
	for _, w := range made {
		if err := g.WorktreeRemove(w.main, w.path, true); err != nil {
			fmt.Fprintf(out, "warn    could not remove %s: %v\n", w.path, err)
			continue
		}
		if err := g.DeleteBranch(w.main, task); err != nil {
			fmt.Fprintf(out, "warn    branch %s left in %s\n", task, w.repo.DirName())
		}
	}
	_ = os.Remove(taskDir) // only succeeds when empty
}

package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/disun001/vantage/internal/git"
)

// RemoveTask removes a task's worktrees and its dir. Without force it first
// refuses if any repo has uncommitted, unpushed or unmerged work, and nothing
// is touched. A task branch is deleted only when fully merged (never -D);
// otherwise it is kept and reported.
func RemoveTask(root string, p Project, g git.Client, task string, force bool, out io.Writer) error {
	if err := ValidTaskName(task); err != nil {
		return err
	}
	if _, err := os.Stat(p.TaskPath(root, task)); err != nil {
		return fmt.Errorf("task %q not found", task)
	}
	repos := TaskRepos(root, p, task)
	if !force {
		if problems := taskProblems(root, p, g, task, repos); len(problems) > 0 {
			return fmt.Errorf("task %q has work that would be lost (use --force):\n  %s",
				task, strings.Join(problems, "\n  "))
		}
	}
	var errs []error
	for _, r := range repos {
		if err := removeRepoTask(root, p, g, task, r, force, out); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.DirName(), err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if err := os.Remove(p.TaskPath(root, task)); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(out, "warn    %s not removed: %v\n", p.TaskPath(root, task), err)
		return nil
	}
	_ = os.Remove(p.TasksPath(root)) // only when empty
	fmt.Fprintf(out, "removed task %s\n", task)
	warnWorkspace(p.Name, task, out)
	return nil
}

func removeRepoTask(root string, p Project, g git.Client, task string, r Repo, force bool, out io.Writer) error {
	main := p.RepoPath(root, r)
	if err := g.WorktreeRemove(main, p.TaskRepoPath(root, task, r), force); err != nil {
		return err
	}
	fmt.Fprintf(out, "removed %s\n", r.DirName())
	if !g.RefExists(main, "refs/heads/"+task) {
		return nil
	}
	if err := g.DeleteBranch(main, task); err != nil {
		fmt.Fprintf(out, "kept    branch %s in %s (not fully merged)\n", task, r.DirName())
	}
	return nil
}

// taskProblems lists, per repo, the work that removing the task would lose.
func taskProblems(root string, p Project, g git.Client, task string, repos []Repo) []string {
	var out []string
	for _, r := range repos {
		for _, why := range repoProblems(g, p.TaskRepoPath(root, task, r), task) {
			out = append(out, r.DirName()+": "+why)
		}
	}
	return out
}

func repoProblems(g git.Client, wt, task string) []string {
	st, err := g.Status(wt)
	if err != nil {
		return []string{"cannot read status: " + err.Error()}
	}
	var why []string
	if st.Dirty {
		why = append(why, "uncommitted changes")
	}
	if !g.RefExists(wt, "refs/heads/"+task) {
		return why
	}
	if n, err := g.Unpushed(wt, task); err != nil {
		why = append(why, "cannot count unpushed commits: "+err.Error())
	} else if n > 0 {
		why = append(why, fmt.Sprintf("%d unpushed commit(s)", n))
	}
	if st.Default == "" {
		return append(why, "default branch unknown, cannot check merge state")
	}
	if n, err := g.Unmerged(wt, task, "origin/"+st.Default); err != nil {
		why = append(why, "cannot check merge state: "+err.Error())
	} else if n > 0 {
		why = append(why, fmt.Sprintf("%d commit(s) not merged into %s", n, st.Default))
	}
	return why
}

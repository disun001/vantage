package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/disun001/vantage/internal/git"
)

// Remove deletes a project: every task (worktrees and dir), the main clones
// and the definition file. Without force it first refuses if any task or
// clone has work that would be lost, and nothing is touched. The definition
// file goes last, so a failure part way can be retried. Workspace files are
// never deleted: they may hold edits.
func Remove(root string, p Project, g git.Client, force bool, out io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	file, err := PathOf(p.Name)
	if err != nil {
		return err
	}
	tasks, err := ListTasks(root, p)
	if err != nil {
		return err
	}
	if !force {
		if problems := projectProblems(root, p, g, tasks); len(problems) > 0 {
			return fmt.Errorf("project %q has work that would be lost (use --force):\n  %s",
				p.Name, strings.Join(problems, "\n  "))
		}
	}
	for _, t := range tasks {
		if err := RemoveTask(root, p, g, t, force, out); err != nil {
			return fmt.Errorf("task %s: %w", t, err)
		}
	}
	if err := removeClones(root, p, out); err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintf(out, "removed project %s\n", p.Name)
	for _, t := range tasks {
		warnWorkspace(p.Name, t, out)
	}
	return nil
}

// projectProblems lists the work that removing the project would lose.
func projectProblems(root string, p Project, g git.Client, tasks []string) []string {
	var out []string
	for _, t := range tasks {
		for _, pr := range taskProblems(root, p, g, t, TaskRepos(root, p, t)) {
			out = append(out, "task "+t+" "+pr)
		}
	}
	for _, r := range p.Repos {
		dir := p.RepoPath(root, r)
		if ok, _ := isClone(dir); !ok {
			continue
		}
		st, err := g.Status(dir)
		switch {
		case err != nil:
			out = append(out, r.DirName()+": cannot read status: "+err.Error())
		case st.Dirty:
			out = append(out, r.DirName()+": uncommitted changes")
		case st.Ahead > 0:
			out = append(out, fmt.Sprintf("%s: %d unpushed commit(s)", r.DirName(), st.Ahead))
		}
	}
	return out
}

// removeClones deletes the main clones, then the project dir when empty. A
// flat project's clone is the project dir itself. Every path is checked to be
// inside root first, because RemoveAll cannot be undone.
func removeClones(root string, p Project, out io.Writer) error {
	var errs []error
	for _, r := range p.Repos {
		dir := p.RepoPath(root, r)
		if !inside(root, dir) {
			errs = append(errs, fmt.Errorf("%s: path escapes the projects root", r.DirName()))
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.DirName(), err))
			continue
		}
		fmt.Fprintf(out, "removed %s\n", r.DirName())
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	_ = os.Remove(p.Path(root)) // only succeeds when empty
	return nil
}

// inside reports whether path is strictly below root.
func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

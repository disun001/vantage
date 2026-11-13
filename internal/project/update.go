package project

import (
	"errors"
	"fmt"
	"io"

	"github.com/disun001/vantage/internal/git"
)

// Update fetches every cloned repo of p, then fast-forwards the ones that are
// clean and on their default branch. Anything else is reported and left
// alone: nothing is reset or merged. A failing repo does not stop the others;
// the errors come back joined.
func Update(root string, p Project, g git.Client, out io.Writer) error {
	var errs []error
	for _, r := range p.Repos {
		if err := updateRepo(p.RepoPath(root, r), r.DirName(), g, out); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.DirName(), err))
		}
	}
	return errors.Join(errs...)
}

func updateRepo(dir, name string, g git.Client, out io.Writer) error {
	cloned, err := isClone(dir)
	if err != nil {
		return err
	}
	if !cloned {
		fmt.Fprintf(out, "skip    %s (not cloned)\n", name)
		return nil
	}
	if err := g.Fetch(dir); err != nil {
		return err
	}
	st, err := g.Status(dir)
	if err != nil {
		return err
	}
	if why := notFastForwardable(st); why != "" {
		fmt.Fprintf(out, "skip    %s (%s)\n", name, why)
		return nil
	}
	if st.Behind == 0 {
		fmt.Fprintf(out, "ok      %s (up to date)\n", name)
		return nil
	}
	if err := g.FastForward(dir); err != nil {
		return err
	}
	fmt.Fprintf(out, "updated %s (+%d commits)\n", name, st.Behind)
	return nil
}

// notFastForwardable says why a clone must not be touched, "" if it may be.
func notFastForwardable(st git.Status) string {
	switch {
	case st.Branch == "":
		return "detached HEAD"
	case st.Default == "" || st.Branch != st.Default:
		return fmt.Sprintf("on %s, not the default branch", st.Branch)
	case st.Dirty:
		return "uncommitted changes"
	case !st.HasUpstream:
		return "no upstream"
	case st.Ahead > 0 && st.Behind > 0:
		return "diverged from upstream"
	}
	return ""
}

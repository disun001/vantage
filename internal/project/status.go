package project

import (
	"fmt"

	"github.com/disun001/vantage/internal/git"
)

// RepoStatus is the state of one main clone.
type RepoStatus struct {
	Repo   Repo
	Cloned bool
	Git    git.Status
	// Err is why the status could not be read (not cloned is not an error).
	Err error
	// Warnings flag a main clone that is not a pristine base. They are
	// information only; nothing is blocked or reset.
	Warnings []string
}

// Check reads the status of every repo of p under root, in project order.
func Check(root string, p Project, g git.Client) []RepoStatus {
	out := make([]RepoStatus, 0, len(p.Repos))
	for _, r := range p.Repos {
		rs := RepoStatus{Repo: r}
		cloned, err := isClone(p.RepoPath(root, r))
		switch {
		case err != nil:
			rs.Err = err
		case cloned:
			rs.Cloned = true
			rs.Git, rs.Err = g.Status(p.RepoPath(root, r))
			if rs.Err == nil {
				rs.Warnings = warnings(rs.Git)
			}
		}
		out = append(out, rs)
	}
	return out
}

// warnings lists why a main clone is not a pristine base.
func warnings(st git.Status) []string {
	var w []string
	switch {
	case st.Branch == "":
		w = append(w, "detached HEAD")
	case st.Default != "" && st.Branch != st.Default:
		w = append(w, fmt.Sprintf("on %s, default is %s", st.Branch, st.Default))
	}
	if st.Dirty {
		w = append(w, "uncommitted changes")
	}
	return w
}

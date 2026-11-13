package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/disun001/vantage/internal/git"
)

// Sync clones the repos of p that are missing under root and reports each to
// out. Existing clones are left alone. A failing repo does not stop the
// others; the errors come back joined.
func Sync(root string, p Project, g git.Client, out io.Writer) error {
	var errs []error
	for _, r := range p.Repos {
		dest := p.RepoPath(root, r)
		switch cloned, err := isClone(dest); {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", r.DirName(), err))
		case cloned:
			fmt.Fprintf(out, "ok      %s (already cloned)\n", r.DirName())
		default:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := g.Clone(r.URL, dest); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", r.DirName(), err))
				continue
			}
			fmt.Fprintf(out, "cloned  %s\n", r.DirName())
		}
	}
	return errors.Join(errs...)
}

// isClone reports whether dir is a git checkout. A path that exists but is
// not one is an error: sync must never clone over someone's files.
func isClone(dir string) (bool, error) {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false, fmt.Errorf("%s exists but is not a git repo", dir)
	}
	return true, nil
}

package project

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/disun001/vantage/internal/git"
)

// copyFiles copies the untracked files matching patterns (relative globs)
// from the main clone into a task worktree. Only regular files that really
// live inside the clone are copied: symlinks, directories and anything under
// .git are skipped, and an existing destination is never overwritten.
// Contents are never printed, only names.
func copyFiles(g git.Client, label, mainDir, wtDir string, patterns []string, out io.Writer) error {
	realMain, err := filepath.EvalSymlinks(mainDir)
	if err != nil {
		return err
	}
	var errs []error
	for _, pat := range patterns {
		matches, err := fs.Glob(os.DirFS(mainDir), pat)
		if err != nil {
			errs = append(errs, fmt.Errorf("copy %q: %w", pat, err))
			continue
		}
		for _, rel := range matches {
			if err := copyOne(g, label, realMain, mainDir, wtDir, rel, out); err != nil {
				errs = append(errs, fmt.Errorf("copy %s/%s: %w", label, rel, err))
			}
		}
	}
	return errors.Join(errs...)
}

func copyOne(g git.Client, label, realMain, mainDir, wtDir, rel string, out io.Writer) error {
	if underGit(rel) {
		return nil
	}
	src := filepath.Join(mainDir, filepath.FromSlash(rel))
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !insideReal(realMain, src) {
		fmt.Fprintf(out, "warn    %s/%s not copied (not a regular file inside the repo)\n", label, rel)
		return nil
	}
	if g.IsTracked(mainDir, rel) {
		fmt.Fprintf(out, "warn    %s/%s is tracked by git, not copied\n", label, rel)
		return nil
	}
	dst := filepath.Join(wtDir, filepath.FromSlash(rel))
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	if err := writeCopy(src, dst, info.Mode().Perm()); err != nil {
		return err
	}
	fmt.Fprintf(out, "copied  %s/%s\n", label, rel)
	return nil
}

func underGit(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

// insideReal reports whether src, with symlinks resolved, is inside realRoot.
func insideReal(realRoot, src string) bool {
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return false
	}
	return strings.HasPrefix(resolved, realRoot+string(filepath.Separator))
}

func writeCopy(src, dst string, perm fs.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // read-only: a close error loses nothing
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(f, in)
	return err
}

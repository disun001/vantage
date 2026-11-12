package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

// BackupSuffix is added to a workspace file's name for the copy StripSSH
// takes before it rewrites the file. The result does not end in ".toml", so
// List ignores it.
const BackupSuffix = ".vantage.bak"

var (
	windowHeader = regexp.MustCompile(`^[ \t]*\[\[[ \t]*window[ \t]*\]\][ \t]*(?:#.*)?$`)
	paneHeader   = regexp.MustCompile(`^[ \t]*\[\[[ \t]*window\.pane[ \t]*\]\][ \t]*(?:#.*)?$`)
	layoutLine   = regexp.MustCompile(`^[ \t]*layout[ \t]*=`)
)

// eachWorkspace runs fn on the path of every saved workspace, and returns the
// names fn reported as changed. Failures are joined, not fatal.
func eachWorkspace(fn func(path string) (bool, error)) ([]string, error) {
	names, err := List()
	if err != nil {
		return nil, err
	}
	var done []string
	var failed []error
	for _, n := range names {
		path, err := PathOf(n)
		if err != nil {
			continue
		}
		changed, err := fn(path)
		if err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", n, err))
			continue
		}
		if changed {
			done = append(done, n)
		}
	}
	return done, errors.Join(failed...)
}

// StripSSH removes the panes that connect to the ssh alias from every saved
// workspace that has one, and returns the names it changed. The files are
// edited in place, so comments and layout survive. A window left without panes
// goes too, and a window that merely loses panes loses its layout line,
// because a saved layout fits one pane count only and restore would fail on
// the new one.
//
// A workspace that would be left with no windows is not touched and is named
// in the error: the format needs at least one pane. So is a file that cannot
// be read, or whose rewrite does not load back as exactly the expected
// workspace. Each rewrite keeps the old file as name.toml + BackupSuffix and
// reaches the file by an atomic rename.
func StripSSH(alias string) ([]string, error) {
	if !aliasRe.MatchString(alias) {
		return nil, fmt.Errorf("invalid ssh alias %q", alias)
	}
	return eachWorkspace(func(path string) (bool, error) { return stripFile(path, alias) })
}

// stripFile rewrites one file. It reports false when the file has no pane for
// the alias.
func stripFile(path, alias string) (bool, error) {
	return rewriteFile(path, alias,
		func(lines []string) ([]string, error) { return stripLines(lines, alias) },
		func(ws Workspace) Workspace { return expectedAfterStrip(ws, alias) })
}

// rewriteFile applies edit to the lines of a workspace file that has a pane
// for alias. The result must load as exactly what expect says, or nothing is
// written. It reports false when the file has no pane for the alias.
func rewriteFile(path, alias string, edit func([]string) ([]string, error), expect func(Workspace) Workspace) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	old, err := Load(path)
	if err != nil {
		return false, err
	}
	if !usesSSH(old, alias) {
		return false, nil
	}
	text := string(data)
	if strings.Contains(text, "\r") {
		return false, errors.New("CRLF line endings; edit it by hand")
	}
	next, err := edit(strings.Split(strings.TrimSuffix(text, "\n"), "\n"))
	if err != nil {
		return false, err
	}
	out := strings.Join(next, "\n") + "\n"
	got, err := parse(out, path)
	if err != nil {
		return false, fmt.Errorf("the edited file does not load: %w", err)
	}
	if !reflect.DeepEqual(got, expect(old)) {
		return false, errors.New("the edit would change more than the ssh panes; edit it by hand")
	}
	st, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := replaceFile(path, out, st.Mode().Perm(), text); err != nil {
		return false, err
	}
	return true, nil
}

// expectedAfterStrip is the workspace the rewritten file must load as.
func expectedAfterStrip(ws Workspace, alias string) Workspace {
	out := ws
	out.Windows = nil
	for _, w := range ws.Windows {
		nw := w
		nw.Panes = nil
		for _, p := range w.Panes {
			if p.SSH != alias {
				nw.Panes = append(nw.Panes, p)
			}
		}
		if len(nw.Panes) == 0 {
			continue
		}
		if len(nw.Panes) != len(w.Panes) {
			nw.Layout = ""
		}
		out.Windows = append(out.Windows, nw)
	}
	return out
}

// replaceFile backs up the old text, writes content next to path, and renames
// it over the file, unless the file no longer holds old.
func replaceFile(path, content string, mode os.FileMode, old string) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".strip-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name()) // nothing was renamed, so nothing is lost
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err = tmp.WriteString(content); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.WriteFile(path+BackupSuffix, []byte(old), 0o600); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	cur, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(cur) != old {
		return errors.New("the file changed while editing; nothing written")
	}
	return os.Rename(tmp.Name(), path)
}

type section struct{ start, end int } // lines [start, end)

// tomlWindow is one [[window]] table of a workspace file as line ranges.
type tomlWindow struct {
	section
	bodyEnd int // first pane header, or the end of the window
	panes   []section
}

// scanWindows finds the window and pane tables of a workspace file. Load has
// already accepted the file, so no other kind of table can be in it.
func scanWindows(lines []string) []tomlWindow {
	var hdrs []int
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "[") {
			hdrs = append(hdrs, i)
		}
	}
	endAt := func(i int) int { // the next header after line i
		for _, h := range hdrs {
			if h > i {
				return h
			}
		}
		return len(lines)
	}
	var wins []tomlWindow
	for _, h := range hdrs {
		switch {
		case windowHeader.MatchString(lines[h]):
			wins = append(wins, tomlWindow{section: section{h, endAt(h)}, bodyEnd: -1})
		case paneHeader.MatchString(lines[h]) && len(wins) > 0:
			w := &wins[len(wins)-1]
			if w.bodyEnd < 0 {
				w.bodyEnd = h
			}
			w.panes = append(w.panes, section{h, endAt(h)})
			w.end = endAt(h) // a window runs to the end of its last pane
		}
	}
	for i := range wins {
		if wins[i].bodyEnd < 0 {
			wins[i].bodyEnd = wins[i].end
		}
	}
	return wins
}

// doomedPanes are the panes of w with an `ssh = "alias"` line.
func doomedPanes(lines []string, w tomlWindow, alias string) []section {
	q := regexp.QuoteMeta(alias)
	sshLine := regexp.MustCompile(`^[ \t]*ssh[ \t]*=[ \t]*(?:"` + q + `"|'` + q + `')[ \t]*(?:#.*)?$`)
	var out []section
	for _, p := range w.panes {
		for i := p.start + 1; i < p.end; i++ {
			if sshLine.MatchString(lines[i]) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// markRemoved flags the lines of s for deletion, except a comment right above
// the next table: it belongs to that table.
func markRemoved(lines []string, drop map[int]bool, s section) {
	end := s.end
	for end < len(lines) && end > s.start+1 && strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#") {
		end--
	}
	for i := s.start; i < end; i++ {
		drop[i] = true
	}
}

// stripLines removes the ssh panes of alias from the lines of a workspace
// file. See StripSSH for what else goes.
func stripLines(lines []string, alias string) ([]string, error) {
	wins := scanWindows(lines)
	drop := map[int]bool{}
	kept, lastRemoved := 0, false
	for wi, w := range wins {
		doomed := doomedPanes(lines, w, alias)
		switch {
		case len(doomed) == 0:
			kept++
		case len(doomed) == len(w.panes):
			markRemoved(lines, drop, w.section)
			lastRemoved = wi == len(wins)-1
		default:
			for _, p := range doomed {
				markRemoved(lines, drop, p)
			}
			for i := w.start + 1; i < w.bodyEnd; i++ {
				if layoutLine.MatchString(lines[i]) {
					drop[i] = true // a saved layout fits the old pane count only
				}
			}
			kept++
		}
	}
	if kept == 0 {
		return nil, errors.New("it would be left with no windows; delete or edit it by hand")
	}
	var out []string
	for i, l := range lines {
		if !drop[i] {
			out = append(out, l)
		}
	}
	if lastRemoved {
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
	}
	return out, nil
}

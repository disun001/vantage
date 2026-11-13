package workspace

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
)

var windowNameLine = regexp.MustCompile(`^([ \t]*name[ \t]*=[ \t]*)(?:"(?:[^"\\]|\\.)*"|'[^']*')([ \t]*(?:#.*)?)$`)

// RenameWindow sets the name of window number pos (counting from 0, in file
// order) of the saved workspace. The window must be called from now, or have
// no name; anything else means the file and the session have drifted apart,
// so the file is left alone. Only the name line changes (or is added), so
// comments and layout survive. The rewrite must load back as exactly the old
// workspace with the one new name, or nothing is written. The old file is
// kept as name.toml + BackupSuffix.
func RenameWindow(workspace string, pos int, from, to string) error {
	if strings.TrimSpace(to) == "" || strings.ContainsFunc(to, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return fmt.Errorf("invalid window name %q", to)
	}
	path, err := PathOf(workspace)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	old, err := Load(path)
	if err != nil {
		return err
	}
	if pos < 0 || pos >= len(old.Windows) {
		return fmt.Errorf("the file has %d windows, the session has more", len(old.Windows))
	}
	if n := old.Windows[pos].Name; n != "" && n != from {
		return fmt.Errorf("window %d is %q in the file", pos+1, n)
	}
	text := string(data)
	if strings.Contains(text, "\r") {
		return errors.New("CRLF line endings; edit it by hand")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	w := scanWindows(lines)[pos]
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(to) + `"`
	done := false
	for i := w.start + 1; i < w.bodyEnd && !done; i++ {
		if m := windowNameLine.FindStringSubmatch(lines[i]); m != nil {
			lines[i], done = m[1]+quoted+m[2], true
		}
	}
	if !done {
		lines = append(lines[:w.start+1], append([]string{"name = " + quoted}, lines[w.start+1:]...)...)
	}
	out := strings.Join(lines, "\n") + "\n"
	got, err := parse(out, path)
	if err != nil {
		return fmt.Errorf("the edited file does not load: %w", err)
	}
	want := old
	want.Windows = append([]Window{}, old.Windows...)
	want.Windows[pos].Name = to
	if !reflect.DeepEqual(got, want) {
		return errors.New("the edit would change more than the window name; edit it by hand")
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	return replaceFile(path, out, st.Mode().Perm(), text)
}

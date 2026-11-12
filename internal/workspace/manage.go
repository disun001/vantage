package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidName reports whether s can name a new workspace.
func ValidName(s string) bool { return nameRe.MatchString(s) }

// PathOf is the file of the workspace called name in Dir. Unlike Resolve it
// never treats name as a path, so a name cannot point outside Dir.
func PathOf(name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid workspace name %q", name)
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".toml"), nil
}

// Exists reports whether a workspace file called name is in Dir.
func Exists(name string) (bool, error) {
	path, err := PathOf(name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Delete removes the workspace file called name.
func Delete(name string) error {
	path, err := PathOf(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no saved workspace %q", name)
	} else if err != nil {
		return err
	}
	return nil
}

// nameLine matches a top-level `name = "..."` line.
var nameLine = regexp.MustCompile(`(?m)^name\s*=\s*(?:"[^"\n]*"|'[^'\n]*')[ \t]*$`)

// Rename moves the workspace file from to to and updates its name. The file
// is edited in place rather than re-encoded, so comments and layout survive.
// It refuses to replace an existing workspace.
func Rename(from, to string) (err error) {
	if !ValidName(to) {
		return fmt.Errorf("invalid workspace name %q (use letters, digits, '-', '_')", to)
	}
	src, err := PathOf(from)
	if err != nil {
		return err
	}
	dst, err := PathOf(to)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no saved workspace %q", from)
	}
	if err != nil {
		return err
	}
	// Only the first match counts: it is the workspace's own name, since
	// window and pane tables come later in the file.
	if loc := nameLine.FindIndex(data); loc != nil && !bytes.Contains(data[:loc[0]], []byte("\n[")) && !bytes.HasPrefix(data, []byte("[")) {
		data = append(append(append([]byte{}, data[:loc[0]]...), fmt.Sprintf("name = %q", to)...), data[loc[1]:]...)
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("workspace %q already exists", to)
	}
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	defer func() {
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	if werr != nil {
		return werr
	}
	// The copy must load under the new name, or the explicit name field was
	// written in a form the line edit did not recognise.
	ws, err := Load(dst)
	if err != nil {
		return err
	}
	if ws.Name != to {
		return fmt.Errorf("could not set the name inside %s; rename it by hand", filepath.Base(src))
	}
	return os.Remove(src)
}

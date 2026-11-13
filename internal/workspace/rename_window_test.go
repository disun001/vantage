package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const windowFixture = `name = "proj"
root = "~/code"

# editor
[[window]]
name = "edit" # main
layout = "tiled"
[[window.pane]]
cwd = "."

[[window]]
[[window.pane]]
cwd = "."
`

func TestRenameWindowEditsOnlyTheNameLine(t *testing.T) {
	dir := withConfig(t, map[string]string{"proj": windowFixture})
	if err := RenameWindow("proj", 0, "edit", `my "ed"`); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "proj.toml"))
	want := strings.Replace(windowFixture, `name = "edit"`, `name = "my \"ed\""`, 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "proj.toml"+BackupSuffix)); err != nil || string(b) != windowFixture {
		t.Errorf("backup: %v", err)
	}
}

func TestRenameWindowAddsMissingName(t *testing.T) {
	dir := withConfig(t, map[string]string{"proj": windowFixture})
	if err := RenameWindow("proj", 1, "zsh", "logs"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "proj.toml"))
	want := strings.Replace(windowFixture, "[[window]]\n[[window.pane]]", "[[window]]\nname = \"logs\"\n[[window.pane]]", 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenameWindowRefusesDriftAndBadNames(t *testing.T) {
	dir := withConfig(t, map[string]string{"proj": windowFixture})
	for _, c := range []struct {
		pos      int
		from, to string
	}{
		{0, "other", "x"}, // the file names it differently
		{5, "edit", "x"},  // the session has more windows than the file
		{0, "edit", ""},
		{0, "edit", "a\nb"},
	} {
		if err := RenameWindow("proj", c.pos, c.from, c.to); err == nil {
			t.Errorf("RenameWindow(%+v) succeeded", c)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "proj.toml")); string(got) != windowFixture {
		t.Error("a refused rename changed the file")
	}
	if _, err := os.Stat(filepath.Join(dir, "proj.toml"+BackupSuffix)); err == nil {
		t.Error("backup for an untouched file")
	}
}

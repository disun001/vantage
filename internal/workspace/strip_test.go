package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const stripFixture = `# my project
name = "proj"
root = "~"

[[window]]
name = "edit"
layout = "main-vertical"

  [[window.pane]]
  cwd = "."

  [[window.pane]]
  ssh = "web"   # the box
  split = "h"

  [[window.pane]]
  ssh = "db"

# remote stuff
[[window]]
name = "remote"
layout = "tiled"

  [[window.pane]]
  ssh = "web"

[[window]]
name = "last"

  [[window.pane]]
  cwd = "/tmp"
`

func withConfig(t *testing.T, files map[string]string) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", cfg)
	dir := filepath.Join(cfg, "workspaces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for n, body := range files {
		if err := os.WriteFile(filepath.Join(dir, n+".toml"), []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestStripSSHRemovesPanesAndEmptyWindows(t *testing.T) {
	dir := withConfig(t, map[string]string{
		"proj":  stripFixture,
		"other": "name = \"other\"\n[[window]]\nname = \"w\"\n[[window.pane]]\nssh = \"db\"\n",
	})
	done, err := StripSSH("web")
	if err != nil || !reflect.DeepEqual(done, []string{"proj"}) {
		t.Fatalf("done=%v err=%v", done, err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "proj.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `# my project
name = "proj"
root = "~"

[[window]]
name = "edit"

  [[window.pane]]
  cwd = "."

  [[window.pane]]
  ssh = "db"

# remote stuff
[[window]]
name = "last"

  [[window.pane]]
  cwd = "/tmp"
`
	// "# remote stuff" sits above the removed window, so it belongs to the
	// previous block's tail and stays.
	if string(got) != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
	bak, err := os.ReadFile(filepath.Join(dir, "proj.toml"+BackupSuffix))
	if err != nil || string(bak) != stripFixture {
		t.Errorf("backup wrong: %v\n%s", err, bak)
	}
	st, _ := os.Stat(filepath.Join(dir, "proj.toml"))
	if st.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
	if other, _ := os.ReadFile(filepath.Join(dir, "other.toml")); !strings.Contains(string(other), `ssh = "db"`) {
		t.Error("an unrelated workspace was touched")
	}
	if _, err := os.Stat(filepath.Join(dir, "other.toml"+BackupSuffix)); err == nil {
		t.Error("backup made for an untouched file")
	}
	names, _ := List()
	if !reflect.DeepEqual(names, []string{"other", "proj"}) {
		t.Errorf("backup files must not be listed as workspaces: %v", names)
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".strip-*"))
	if len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
	ws, err := Load(filepath.Join(dir, "proj.toml"))
	if err != nil || len(ws.Windows) != 2 || ws.Windows[0].Layout != "" || len(ws.Windows[0].Panes) != 2 {
		t.Errorf("loaded %+v, %v", ws, err)
	}
}

func TestStripSSHLeavesEmptiedWorkspaceAlone(t *testing.T) {
	only := "name = \"solo\"\n[[window]]\nname = \"w\"\n[[window.pane]]\nssh = \"web\"\n"
	dir := withConfig(t, map[string]string{"solo": only, "ok": stripFixture})
	done, err := StripSSH("web")
	if err == nil || !strings.Contains(err.Error(), "solo") || !strings.Contains(err.Error(), "no windows") {
		t.Errorf("err = %v", err)
	}
	if !reflect.DeepEqual(done, []string{"ok"}) {
		t.Errorf("one broken workspace must not block the others: %v", done)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "solo.toml")); string(got) != only {
		t.Errorf("solo changed:\n%s", got)
	}
}

func TestStripSSHSkipsUnreadableAndBadAlias(t *testing.T) {
	dir := withConfig(t, map[string]string{"bad": "not toml [[[", "ok": stripFixture})
	done, err := StripSSH("web")
	if err == nil || !strings.Contains(err.Error(), "bad") || !reflect.DeepEqual(done, []string{"ok"}) {
		t.Errorf("done=%v err=%v", done, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "bad.toml")); string(got) != "not toml [[[" {
		t.Error("unreadable file was changed")
	}
	for _, a := range []string{"", "-x", "a b", "a\nb"} {
		if _, err := StripSSH(a); err == nil {
			t.Errorf("StripSSH(%q) accepted", a)
		}
	}
}

func TestStripSSHWhenNothingUsesIt(t *testing.T) {
	dir := withConfig(t, map[string]string{"proj": stripFixture})
	done, err := StripSSH("nobody")
	if err != nil || len(done) != 0 {
		t.Errorf("done=%v err=%v", done, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "proj.toml")); string(got) != stripFixture {
		t.Error("file rewritten although nothing used the alias")
	}
}

func TestStripSSHRefusesWhatItCannotRewriteExactly(t *testing.T) {
	// An inline form the line editor does not understand: the verification
	// must catch that the pane survived and leave the file alone.
	inline := "name = \"x\"\n[[window]]\nname = \"w\"\npane = [{ ssh = \"web\" }, { cwd = \"/\" }]\n"
	dir := withConfig(t, map[string]string{"x": inline})
	_, err := StripSSH("web")
	if err == nil {
		t.Fatal("rewrote a file it did not understand")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "x.toml")); string(got) != inline {
		t.Errorf("file changed:\n%s", got)
	}
}

func TestStripSSHKeepsDefaultNameFiles(t *testing.T) {
	body := "[[window]]\nname = \"w\"\n[[window.pane]]\ncwd = \"/\"\n[[window.pane]]\nssh = \"web\"\n"
	dir := withConfig(t, map[string]string{"noname": body})
	done, err := StripSSH("web")
	if err != nil || !reflect.DeepEqual(done, []string{"noname"}) {
		t.Fatalf("done=%v err=%v", done, err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "noname.toml"))
	if string(got) != "[[window]]\nname = \"w\"\n[[window.pane]]\ncwd = \"/\"\n" {
		t.Errorf("file:\n%s", got)
	}
}

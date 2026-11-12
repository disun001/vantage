package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRenameSSHChangesOnlyTheValue(t *testing.T) {
	dir := withConfig(t, map[string]string{
		"proj":  stripFixture + "\n[[window]]\nname = \"q\"\n[[window.pane]]\nssh = 'web'\n",
		"other": "name = \"other\"\n[[window]]\nname = \"w\"\n[[window.pane]]\nssh = \"db\"\n",
	})
	done, err := RenameSSH("web", "web2")
	if err != nil || !reflect.DeepEqual(done, []string{"proj"}) {
		t.Fatalf("done=%v err=%v", done, err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "proj.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(stripFixture, `ssh = "web"`, `ssh = "web2"`) +
		"\n[[window]]\nname = \"q\"\n[[window.pane]]\nssh = 'web2'\n"
	if string(got) != want {
		t.Errorf("proj.toml:\n%s\nwant:\n%s", got, want)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "proj.toml"+BackupSuffix)); err != nil || !strings.Contains(string(b), `ssh = "web"`) {
		t.Errorf("backup: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "other.toml")); !strings.Contains(string(b), `ssh = "db"`) {
		t.Error("an unrelated workspace changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "other.toml"+BackupSuffix)); err == nil {
		t.Error("backup for an untouched file")
	}
	if u, _ := UsingSSH("web"); len(u) != 0 {
		t.Errorf("still using web: %v", u)
	}
	if u, _ := UsingSSH("web2"); !reflect.DeepEqual(u, []string{"proj"}) {
		t.Errorf("using web2: %v", u)
	}
}

func TestRenameSSHRefusesBadAliasesAndSkipsBadFiles(t *testing.T) {
	dir := withConfig(t, map[string]string{"proj": stripFixture, "broken": "not toml ["})
	if _, err := RenameSSH("web", "-oX"); err == nil {
		t.Error("an option-like alias was accepted")
	}
	if _, err := RenameSSH("web", "a b"); err == nil {
		t.Error("an alias with a blank was accepted")
	}
	done, err := RenameSSH("web", "web2")
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("the unreadable file is not named: %v", err)
	}
	if !reflect.DeepEqual(done, []string{"proj"}) {
		t.Errorf("done = %v", done)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "broken.toml")); string(b) != "not toml [" {
		t.Error("the broken file was touched")
	}
}

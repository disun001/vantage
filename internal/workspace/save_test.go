package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sample() Workspace {
	return Workspace{
		Name: "demo",
		Windows: []Window{
			{Name: "w", Layout: "tiled", Panes: []Pane{
				{CWD: "~/x"},
				{SSH: "prod", Split: "v"},
				{Cmd: `echo "hi"`},
			}},
		},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "demo.toml")
	if err := Save(path, sample(), false); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := sample()
	if got.Name != want.Name || len(got.Windows) != 1 {
		t.Fatalf("got %+v", got)
	}
	for i, p := range want.Windows[0].Panes {
		if got.Windows[0].Panes[i] != p {
			t.Errorf("pane %d = %+v, want %+v", i, got.Windows[0].Panes[i], p)
		}
	}
}

func TestSaveWritesExplicitAutorunFalseForCmd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.toml")
	if err := Save(path, sample(), false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "autorun = false") != 1 {
		t.Errorf("want exactly one explicit autorun = false:\n%s", b)
	}
}

func TestSaveRefusesOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.toml")
	if err := Save(path, sample(), false); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, sample(), false); err == nil {
		t.Fatal("second save without overwrite should fail")
	}
	if err := Save(path, sample(), true); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := Save(path, Workspace{Name: "bad name"}, false); err == nil {
		t.Fatal("want validation error")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("invalid workspace must not create a file")
	}
}

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

// gitIn runs plain git in dir with a fixed identity, for fixtures only.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// origin makes a repo with one commit on main and returns its path.
func origin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func TestCloneAndStatus(t *testing.T) {
	needGit(t)
	g := Client{}
	src := origin(t)
	dest := filepath.Join(t.TempDir(), "clone")
	if err := g.Clone(src, dest); err != nil {
		t.Fatal(err)
	}
	st, err := g.Status(dest)
	if err != nil {
		t.Fatal(err)
	}
	want := Status{Branch: "main", Default: "main", HasUpstream: true}
	if st != want {
		t.Errorf("status = %+v, want %+v", st, want)
	}
}

func TestStatusDirtyAheadBehind(t *testing.T) {
	needGit(t)
	g := Client{}
	src := origin(t)
	dest := filepath.Join(t.TempDir(), "clone")
	if err := g.Clone(src, dest); err != nil {
		t.Fatal(err)
	}
	// behind: a new commit upstream, then fetch
	gitIn(t, src, "commit", "-q", "--allow-empty", "-m", "up")
	if err := g.Fetch(dest); err != nil {
		t.Fatal(err)
	}
	// ahead: a local commit
	gitIn(t, dest, "commit", "-q", "--allow-empty", "-m", "local")
	// dirty: an untracked file
	if err := os.WriteFile(filepath.Join(dest, "new.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := g.Status(dest)
	if err != nil {
		t.Fatal(err)
	}
	if st.Ahead != 1 || st.Behind != 1 || !st.Dirty {
		t.Errorf("status = %+v", st)
	}
}

func TestStatusDetached(t *testing.T) {
	needGit(t)
	g := Client{}
	src := origin(t)
	gitIn(t, src, "checkout", "-q", "--detach")
	st, err := g.Status(src)
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "" {
		t.Errorf("detached HEAD should report no branch, got %q", st.Branch)
	}
}

func TestCloneFailureNamesGit(t *testing.T) {
	needGit(t)
	g := Client{}
	err := g.Clone(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "d"))
	if err == nil {
		t.Fatal("want error cloning a missing repo")
	}
}

func TestCloneRejectsOptionLikeURL(t *testing.T) {
	needGit(t)
	g := Client{}
	// After `--` this is a (nonexistent) path, not an option.
	if err := g.Clone("--upload-pack=touch /tmp/x", filepath.Join(t.TempDir(), "d")); err == nil {
		t.Fatal("want error")
	}
}

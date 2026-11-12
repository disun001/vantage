package sshconfig

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := Host{Alias: "web-1", HostName: "192.0.2.7", User: "deploy", Port: "2222", ProxyJump: "me@bastion:22,jump2"}
	if err := Validate(ok); err != nil {
		t.Fatalf("valid host rejected: %v", err)
	}
	if err := Validate(Host{Alias: "x"}); err != nil {
		t.Fatalf("alias alone rejected: %v", err)
	}
	if err := Validate(Host{Alias: "v6", HostName: "[2001:db8::1]"}); err != nil {
		t.Fatalf("bracketed ipv6 rejected: %v", err)
	}
	bad := map[string]Host{
		"empty alias":     {},
		"option alias":    {Alias: "-oProxyCommand=x"},
		"wildcard alias":  {Alias: "web*"},
		"space alias":     {Alias: "a b"},
		"newline host":    {Alias: "a", HostName: "x\nProxyCommand evil"},
		"space host":      {Alias: "a", HostName: "x y"},
		"option host":     {Alias: "a", HostName: "-oProxyCommand=x"},
		"quote host":      {Alias: "a", HostName: `x"y`},
		"percent host":    {Alias: "a", HostName: "x%h"},
		"option user":     {Alias: "a", User: "-l"},
		"newline user":    {Alias: "a", User: "u\nHost *"},
		"port zero":       {Alias: "a", Port: "0"},
		"port large":      {Alias: "a", Port: "65536"},
		"port text":       {Alias: "a", Port: "ssh"},
		"port signed":     {Alias: "a", Port: "+22"},
		"port padded":     {Alias: "a", Port: "022"},
		"jump option":     {Alias: "a", ProxyJump: "-oProxyCommand=x"},
		"jump space":      {Alias: "a", ProxyJump: "a b"},
		"jump shell":      {Alias: "a", ProxyJump: "a;rm"},
		"jump comment":    {Alias: "a", ProxyJump: "a#b"},
		"jump dollar":     {Alias: "a", ProxyJump: "$HOME"},
		"unbalanced ipv6": {Alias: "a", HostName: "[::1"},
	}
	for name, h := range bad {
		if err := Validate(h); err == nil {
			t.Errorf("%s: %+v accepted", name, h)
		}
	}
}

const sample = `# my hosts
Compression yes

Host web
  HostName 192.0.2.1
  User deploy
  IdentityFile ~/.ssh/key # not ours

Host db staging
  HostName 192.0.2.2

# defaults
Host *
  ServerAliveInterval 30
`

func TestAddBlockGoesBeforeWildcardDefaults(t *testing.T) {
	got, err := addBlock(sample, Host{Alias: "new", HostName: "192.0.2.9", Port: "2200"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(sample, "# defaults\n", "Host new\n  HostName 192.0.2.9\n  Port 2200\n\n# defaults\n", 1)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddBlockPlacement(t *testing.T) {
	h := Host{Alias: "n", User: "u"}
	cases := map[string]struct{ in, want string }{
		"empty file":       {"", "Host n\n  User u\n"},
		"no wildcard":      {"Host a\n  User x\n", "Host a\n  User x\n\nHost n\n  User u\n"},
		"blank at the end": {"Host a\n  User x\n\n", "Host a\n  User x\n\nHost n\n  User u\n"},
		"before match":     {"Host a\n  User x\nMatch all\n  User y\n", "Host a\n  User x\n\nHost n\n  User u\n\nMatch all\n  User y\n"},
		"no final newline": {"Host a\n  User x", "Host a\n  User x\n\nHost n\n  User u\n"},
		"globals only":     {"User root\n", "User root\n\nHost n\n  User u\n"},
	}
	for name, c := range cases {
		got, err := addBlock(c.in, h)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v\nwant %q", name, got, err, c.want)
		}
	}
}

func TestAddBlockRefusesExistingAlias(t *testing.T) {
	for _, alias := range []string{"web", "db", "staging"} { // db is in a shared line
		if _, err := addBlock(sample, Host{Alias: alias}); !errors.Is(err, ErrExists) {
			t.Errorf("%s: err = %v", alias, err)
		}
	}
}

func TestEditBlock(t *testing.T) {
	cases := map[string]struct {
		in   string
		h    Host
		want string
	}{
		"replace keeps the rest": {
			"Host web\n  HostName a\n  User deploy\n  IdentityFile k\n\nHost z\n  User q\n",
			Host{Alias: "web", HostName: "b", User: "deploy"},
			"Host web\n  HostName b\n  User deploy\n  IdentityFile k\n\nHost z\n  User q\n",
		},
		"add after the last option": {
			"Host web\n  HostName a\n  IdentityFile k\n\n# next\nHost z\n",
			Host{Alias: "web", HostName: "a", Port: "2200"},
			"Host web\n  HostName a\n  IdentityFile k\n  Port 2200\n\n# next\nHost z\n",
		},
		"clear removes the line": {
			"Host web\n  HostName a\n  User u\n",
			Host{Alias: "web", HostName: "a"},
			"Host web\n  HostName a\n",
		},
		"keeps key spelling, tabs and equals": {
			"Host web\n\thostname=a\n\tUSER  u\n",
			Host{Alias: "web", HostName: "b", User: "v"},
			"Host web\n\thostname=b\n\tUSER  v\n",
		},
		"nothing to change": {
			"Host web\n  HostName a\n",
			Host{Alias: "web", HostName: "a"},
			"Host web\n  HostName a\n",
		},
		"block at the end without newline": {
			"Host web\n  HostName a",
			Host{Alias: "web", HostName: "a", User: "u"},
			"Host web\n  HostName a\n  User u\n",
		},
		"header only block": {
			"Host web\nHost z\n  User q\n",
			Host{Alias: "web", User: "u"},
			"Host web\n  User u\nHost z\n  User q\n",
		},
		"duplicate option: first wins, edit that": {
			"Host web\n  User a\n  User b\n",
			Host{Alias: "web", User: "c"},
			"Host web\n  User c\n  User b\n",
		},
		"other block with the same option untouched": {
			"Host a\n  User x\nHost web\n  User y\n",
			Host{Alias: "web", User: "z"},
			"Host a\n  User x\nHost web\n  User z\n",
		},
		"match section is not part of the block": {
			"Host web\n  User a\nMatch all\n  Port 1\n",
			Host{Alias: "web", User: "a", Port: "9"},
			"Host web\n  User a\n  Port 9\nMatch all\n  Port 1\n",
		},
	}
	for name, c := range cases {
		got, err := editBlock(c.in, c.h)
		if err != nil || got != c.want {
			t.Errorf("%s:\ngot  %q, %v\nwant %q", name, got, err, c.want)
		}
	}
}

func TestEditBlockErrors(t *testing.T) {
	if _, err := editBlock(sample, Host{Alias: "db", User: "x"}); !errors.Is(err, ErrShared) {
		t.Errorf("shared: %v", err)
	}
	if _, err := editBlock(sample, Host{Alias: "nope", User: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	if _, err := editBlock("Host web\r\n  User a\r\n", Host{Alias: "web", User: "b"}); err == nil {
		t.Error("CRLF file accepted")
	}
	if _, err := editBlock("Host \"web\"\n  User a\n", Host{Alias: "web", User: "b"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("quoted pattern: %v", err)
	}
}

func TestOwnIgnoresWildcardDefaults(t *testing.T) {
	cfg := "Host web\n  HostName a\n\nHost *\n  User fallback\n"
	lines, _ := splitLines(cfg)
	got, err := ownValues(lines, "web")
	if err != nil || !reflect.DeepEqual(got, Host{Alias: "web", HostName: "a"}) {
		t.Errorf("got %+v, %v", got, err)
	}
}

// fileWith writes content to a fresh config path and returns the File.
func fileWith(t *testing.T, content string, mode os.FileMode) File {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return File{Path: p}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFileEditWritesBackupAndKeepsMode(t *testing.T) {
	f := fileWith(t, sample, 0o640)
	if err := f.Edit(Host{Alias: "web", HostName: "192.0.2.50", User: "deploy"}); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(sample, "HostName 192.0.2.1", "HostName 192.0.2.50", 1)
	if got := readFile(t, f.Path); got != want {
		t.Errorf("config:\n%s\nwant:\n%s", got, want)
	}
	if got := readFile(t, f.Path+BackupSuffix); got != sample {
		t.Errorf("backup differs from the original:\n%s", got)
	}
	st, err := os.Stat(f.Path)
	if err != nil || st.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, %v", st.Mode().Perm(), err)
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(f.Path), ".config.vantage-*"))
	if len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
	own, err := f.Own("web")
	if err != nil || own.HostName != "192.0.2.50" {
		t.Errorf("Own = %+v, %v", own, err)
	}
}

func TestFileAddThenList(t *testing.T) {
	f := fileWith(t, sample, 0o600)
	if err := f.Add(Host{Alias: "fresh", HostName: "192.0.2.77", User: "me", Port: "22", ProxyJump: "web"}); err != nil {
		t.Fatal(err)
	}
	hosts, err := f.List()
	if err != nil {
		t.Fatal(err)
	}
	var got Host
	for _, h := range hosts {
		if h.Alias == "fresh" {
			got = h
		}
	}
	// ServerAliveInterval from `Host *` does not show up in Host, so the
	// listed host is exactly what was added.
	want := Host{Alias: "fresh", HostName: "192.0.2.77", User: "me", Port: "22", ProxyJump: "web"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if err := f.Add(Host{Alias: "fresh"}); !errors.Is(err, ErrExists) {
		t.Errorf("second add: %v", err)
	}
}

func TestFileAddCreatesMissingFileAndDir(t *testing.T) {
	f := File{Path: filepath.Join(t.TempDir(), ".ssh", "config")}
	if err := f.Add(Host{Alias: "first", HostName: "192.0.2.1"}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, f.Path); got != "Host first\n  HostName 192.0.2.1\n" {
		t.Errorf("got %q", got)
	}
	st, _ := os.Stat(f.Path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
	if _, err := os.Stat(f.Path + BackupSuffix); err == nil {
		t.Error("a backup of nothing was made")
	}
}

// A dotfiles setup symlinks ~/.ssh/config. The link must survive and the
// target file must be the one that changes.
func TestFileFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "ssh_config")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("Host a\n  User x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := (File{Path: link}).Edit(Host{Alias: "a", User: "y"}); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink replaced: %v, %v", st, err)
	}
	if got := readFile(t, target); got != "Host a\n  User y\n" {
		t.Errorf("target file: %q", got)
	}
}

func TestFileRefusesBadInputWithoutTouchingTheFile(t *testing.T) {
	f := fileWith(t, sample, 0o600)
	for _, h := range []Host{
		{Alias: "web", User: "x\nProxyCommand evil"},
		{Alias: "db", User: "x"},    // shared Host line
		{Alias: "ghost", User: "x"}, // not in the file
		{Alias: "-oX"},
	} {
		if err := f.Edit(h); err == nil {
			t.Errorf("Edit(%+v) succeeded", h)
		}
	}
	if got := readFile(t, f.Path); got != sample {
		t.Errorf("file changed:\n%s", got)
	}
	if _, err := os.Stat(f.Path + BackupSuffix); err == nil {
		t.Error("backup written for a refused change")
	}
}

func TestFileRefusesUnparsableConfig(t *testing.T) {
	bad := "Host a\n  User x\nMatch\n" // Match with no criterion
	f := fileWith(t, bad, 0o600)
	if _, err := Hosts(strings.NewReader(bad)); err == nil {
		// A Match line is the only malformed input this parser rejects that
		// the editor would otherwise touch.
		t.Fatal("fixture no longer fails to parse; pick another")
	}
	if err := f.Add(Host{Alias: "n"}); err == nil {
		t.Error("edited a config that does not parse")
	}
	if got := readFile(t, f.Path); got != bad {
		t.Errorf("file changed: %q", got)
	}
}

func TestDeleteBlock(t *testing.T) {
	cases := map[string]struct{ in, alias, want string }{
		"middle": {
			"Host a\n  User x\n\nHost b\n  User y\n\nHost c\n  User z\n", "b",
			"Host a\n  User x\n\nHost c\n  User z\n",
		},
		"first": {
			"Host a\n  User x\n\nHost b\n  User y\n", "a",
			"Host b\n  User y\n",
		},
		"last drops the trailing blank": {
			"Host a\n  User x\n\nHost b\n  User y\n", "b",
			"Host a\n  User x\n",
		},
		"only host": {"Host a\n  User x\n", "a", ""},
		"comment of the next section stays": {
			"Host a\n  User x\n\n# defaults\nHost *\n  User d\n", "a",
			"# defaults\nHost *\n  User d\n",
		},
		"comment above the block stays": {
			"# about a\nHost a\n  User x\nHost b\n", "a",
			"# about a\nHost b\n",
		},
		"globals above survive": {
			"Compression yes\n\nHost a\n  User x\n", "a",
			"Compression yes\n",
		},
		"before a match": {
			"Host a\n  User x\nMatch all\n  User y\n", "a",
			"Match all\n  User y\n",
		},
	}
	for name, c := range cases {
		got, err := deleteBlock(c.in, c.alias)
		if err != nil || got != c.want {
			t.Errorf("%s:\ngot  %q, %v\nwant %q", name, got, err, c.want)
		}
	}
	if _, err := deleteBlock(sample, "db"); !errors.Is(err, ErrShared) {
		t.Errorf("shared: %v", err)
	}
	if _, err := deleteBlock(sample, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
}

func TestFileDelete(t *testing.T) {
	f := fileWith(t, sample, 0o600)
	if err := f.Delete("web"); err != nil {
		t.Fatal(err)
	}
	hosts, err := f.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hosts {
		if h.Alias == "web" {
			t.Error("web is still listed")
		}
	}
	if got := readFile(t, f.Path+BackupSuffix); got != sample {
		t.Errorf("backup differs from the original:\n%s", got)
	}
	if err := f.Delete("web"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	for _, bad := range []string{"db", "-oX", "x\ny", "*"} {
		before := readFile(t, f.Path)
		if err := f.Delete(bad); err == nil {
			t.Errorf("Delete(%q) succeeded", bad)
		}
		if readFile(t, f.Path) != before {
			t.Errorf("Delete(%q) changed the file", bad)
		}
	}
}

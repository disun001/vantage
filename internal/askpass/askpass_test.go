package askpass

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/vault"
)

var web = sshconfig.Host{Alias: "web", HostName: "192.0.2.7", User: "deploy"}

func TestPlain(t *testing.T) {
	cases := map[string]struct {
		prompt string
		h      sshconfig.Host
		want   bool
	}{
		"password":                {"deploy@192.0.2.7's password: ", web, true},
		"keyboard-interactive":    {"(deploy@192.0.2.7) Password: ", web, true},
		"no user uses local":      {"me@192.0.2.7's password: ", sshconfig.Host{Alias: "web", HostName: "192.0.2.7"}, true},
		"no host name uses alias": {"deploy@web's password: ", sshconfig.Host{Alias: "web", User: "deploy"}, true},
		"jump host":               {"deploy@bastion's password: ", web, false},
		"other user":              {"root@192.0.2.7's password: ", web, false},
		"key passphrase":          {"Enter passphrase for key '/k': ", web, false},
		"host key":                {"Are you sure you want to continue connecting (yes/no)? ", web, false},
		"verification code":       {"Verification code: ", web, false},
		"empty":                   {"", web, false},
		"prefix trick":            {"x deploy@192.0.2.7's password: ", web, false},
	}
	for name, c := range cases {
		if got := Plain(c.prompt, c.h, "me"); got != c.want {
			t.Errorf("%s: Plain(%q) = %v, want %v", name, c.prompt, got, c.want)
		}
	}
}

func startServer(t *testing.T, store map[string]string) *Listener {
	t.Helper()
	s := Server{User: "me", Password: func(a string) (string, error) {
		if p, ok := store[a]; ok {
			return p, nil
		}
		return "", vault.ErrNoPassword
	}, Host: func(a string) (sshconfig.Host, error) {
		if a == "web" || a == "nopass" {
			h := web
			h.Alias = a
			return h, nil
		}
		return sshconfig.Host{}, errors.New("no such host")
	}}
	l, err := s.Listen("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestServerAnswersOnlyAPlainPasswordPrompt(t *testing.T) {
	l := startServer(t, map[string]string{"web": "s3cret"})
	pw, ok, err := Ask(l.Path, "web", "deploy@192.0.2.7's password: ")
	if err != nil || !ok || pw != "s3cret" {
		t.Fatalf("Ask = %q, %v, %v", pw, ok, err)
	}
	for name, c := range map[string][2]string{
		"other prompt": {"web", "Enter passphrase for key '/k': "},
		"jump host":    {"web", "deploy@bastion's password: "},
		"unknown host": {"nope", "deploy@192.0.2.7's password: "},
		"no password":  {"nopass", "deploy@192.0.2.7's password: "},
	} {
		pw, ok, err := Ask(l.Path, c[0], c[1])
		if err != nil || ok || pw != "" {
			t.Errorf("%s: Ask = %q, %v, %v; want declined", name, pw, ok, err)
		}
	}
}

func TestSocketIsPrivateAndRemovedOnClose(t *testing.T) {
	l := startServer(t, nil)
	st, err := os.Stat(l.Path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("socket mode = %v, %v", st.Mode().Perm(), err)
	}
	dst, err := os.Stat(filepath.Dir(l.Path))
	if err != nil || dst.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v", dst.Mode().Perm(), err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(l.Path)); !os.IsNotExist(err) {
		t.Errorf("the socket dir is still there: %v", err)
	}
	if _, _, err := Ask(l.Path, "web", "x"); err == nil {
		t.Error("a closed server must not be reachable")
	}
}

type fakeTerm struct {
	shown  strings.Builder
	line   string
	secret string
}

func (f *fakeTerm) Show(s string)               { f.shown.WriteString(s) }
func (f *fakeTerm) ReadLine() (string, error)   { return f.line, nil }
func (f *fakeTerm) ReadSecret() (string, error) { return f.secret, nil }

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestRunUsesTheVaultThenTheTerminal(t *testing.T) {
	l := startServer(t, map[string]string{"web": "s3cret"})
	env := envOf(map[string]string{EnvAlias: "web", EnvSocket: l.Path})

	var out bytes.Buffer
	term := &fakeTerm{secret: "typed"}
	if code := Run(env, "deploy@192.0.2.7's password: ", &out, term); code != 0 || out.String() != "s3cret\n" || term.shown.Len() != 0 {
		t.Errorf("vault answer: code %d, out %q, terminal %q", code, out.String(), term.shown.String())
	}

	out.Reset()
	if code := Run(env, "Enter passphrase for key '/k': ", &out, term); code != 0 || out.String() != "typed\n" {
		t.Errorf("terminal answer: code %d, out %q", code, out.String())
	}
	if !strings.Contains(term.shown.String(), "Enter passphrase") {
		t.Errorf("the person never saw the prompt: %q", term.shown.String())
	}
}

func TestRunConfirmAndInfoPromptsGoToTheTerminal(t *testing.T) {
	l := startServer(t, map[string]string{"web": "s3cret"})
	base := map[string]string{EnvAlias: "web", EnvSocket: l.Path}
	cases := map[string]struct {
		kind, line string
		want       int
	}{
		"confirm yes":  {"confirm", "yes", 0},
		"confirm no":   {"confirm", "no", 1},
		"confirm bare": {"confirm", "", 1},
		"info":         {"none", "", 0},
	}
	for name, c := range cases {
		env := map[string]string{EnvPrompt: c.kind}
		for k, v := range base {
			env[k] = v
		}
		var out bytes.Buffer
		if code := Run(envOf(env), "deploy@192.0.2.7's password: ", &out, &fakeTerm{line: c.line}); code != c.want || out.Len() != 0 {
			t.Errorf("%s: code %d, out %q; want %d and nothing on stdout", name, code, out.String(), c.want)
		}
	}
}

func TestRunWithoutAServerAsksTheTerminal(t *testing.T) {
	var out bytes.Buffer
	if code := Run(envOf(nil), "Password: ", &out, &fakeTerm{secret: "typed"}); code != 0 || out.String() != "typed\n" {
		t.Errorf("code %d, out %q", code, out.String())
	}
	dead := envOf(map[string]string{EnvAlias: "web", EnvSocket: filepath.Join(t.TempDir(), "gone")})
	out.Reset()
	if code := Run(dead, "deploy@192.0.2.7's password: ", &out, &fakeTerm{secret: "typed"}); code != 0 || out.String() != "typed\n" {
		t.Errorf("a dead server must fall back to the terminal: code %d, out %q", code, out.String())
	}
}

func newSession(t *testing.T, identity string) (*Session, vault.Vault) {
	t.Helper()
	v := vault.Vault{Path: filepath.Join(t.TempDir(), "p.age"), WorkFactor: 2}
	s := &Session{
		Vault: v, Identity: identity, Bin: "/opt/bin/vantage", User: "me",
		Host: func(string) (sshconfig.Host, error) { return web, nil },
	}
	t.Cleanup(s.Close)
	return s, v
}

func TestSessionUnlocksOnceAndServesStoredPasswords(t *testing.T) {
	s, v := newSession(t, "")
	if s.NeedsUnlock() || s.Stored("web") != None {
		t.Fatal("no vault: nothing to unlock, nothing stored")
	}
	if err := v.Set(vault.Key{Passphrase: "pw"}, "web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if !s.NeedsUnlock() || s.Stored("web") != Locked || s.Askpass("web") != nil {
		t.Fatal("a vault with a passphrase starts locked")
	}
	if err := s.Unlock("wrong"); !errors.Is(err, vault.ErrWrongKey) {
		t.Fatalf("wrong passphrase: %v", err)
	}
	if !s.NeedsUnlock() {
		t.Error("a failed unlock must leave the vault locked")
	}
	if err := s.Unlock("pw"); err != nil {
		t.Fatal(err)
	}
	if s.NeedsUnlock() || s.Stored("web") != Stored || s.Stored("db") != None {
		t.Error("states after unlock")
	}
	a := s.Askpass("web")
	if a == nil || a.Bin != "/opt/bin/vantage" || s.Askpass("db") != nil {
		t.Fatalf("Askpass = %+v", a)
	}
	pw, ok, err := Ask(a.Socket, "web", "deploy@192.0.2.7's password: ")
	if err != nil || !ok || pw != "s3cret" {
		t.Errorf("served %q, %v, %v", pw, ok, err)
	}
	s.Close()
	if _, _, err := Ask(a.Socket, "web", "x"); err == nil {
		t.Error("Close must stop the server")
	}
}

func TestSessionWithAnIdentityNeedsNoPrompt(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(idPath, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, v := newSession(t, idPath)
	if err := v.Set(vault.Key{IdentityFile: idPath}, "web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if s.NeedsUnlock() || s.Stored("web") != Stored || s.Askpass("web") == nil {
		t.Error("an identity vault opens on first use")
	}
}

func TestSessionSetAndDelete(t *testing.T) {
	s, v := newSession(t, "")
	if !s.NeedsCreate() || s.NeedsUnlock() {
		t.Fatal("no vault yet: it must be created, not unlocked")
	}
	if err := s.Set("web", "x"); !errors.Is(err, ErrLocked) {
		t.Fatalf("Set before Unlock: %v", err)
	}
	if err := s.Unlock("fresh"); err != nil { // no file yet: this chooses the passphrase
		t.Fatal(err)
	}
	if err := s.Set("web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if s.Stored("web") != Stored || s.NeedsCreate() {
		t.Error("state after Set")
	}
	if got, err := v.Get(vault.Key{Passphrase: "fresh"}, "web"); err != nil || got != "s3cret" {
		t.Errorf("file holds %q, %v", got, err)
	}
	if err := s.Delete("web"); err != nil || s.Stored("web") != None {
		t.Errorf("Delete: %v, state %v", err, s.Stored("web"))
	}
}

func TestSessionRenameAndDeleteFollowTheHost(t *testing.T) {
	s, _ := newSession(t, "")
	if err := s.Rename("a", "b"); err != nil || s.Delete("a") != nil {
		t.Fatal("with no vault there is nothing to move or remove")
	}
	if err := s.Unlock("pw"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename("web", "web2"); err != nil {
		t.Fatal(err)
	}
	if s.Stored("web") != None || s.Stored("web2") != Stored {
		t.Error("the password must move with the alias")
	}
	if err := s.Rename("nothing", "x"); err != nil {
		t.Errorf("renaming a host without a password: %v", err)
	}
	s.Close()
	if err := s.Rename("web2", "web3"); !errors.Is(err, ErrLocked) {
		t.Errorf("a locked vault with a file must say so, got %v", err)
	}
}

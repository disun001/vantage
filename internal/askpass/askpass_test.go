package askpass

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	v := vault.Vault{Path: filepath.Join(t.TempDir(), "p.age"), WorkFactor: 2}
	k := vault.Key{Passphrase: "pw"}
	for a, p := range store {
		if err := v.Set(k, a, p); err != nil {
			t.Fatal(err)
		}
	}
	s := Server{Vault: v, Key: k, User: "me", Host: func(a string) (sshconfig.Host, error) {
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

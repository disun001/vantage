package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/askpass"
	"github.com/disun001/vantage/internal/vault"
	"github.com/disun001/vantage/internal/workspace"
)

type fakePasswords struct {
	locked bool
	pass   string
	stored map[string]bool
	tried  []string
}

func (f *fakePasswords) NeedsUnlock() bool { return f.locked }

func (f *fakePasswords) Unlock(p string) error {
	f.tried = append(f.tried, p)
	if p != f.pass {
		return vault.ErrWrongKey
	}
	f.locked = false
	return nil
}

func (f *fakePasswords) Askpass(alias string) *workspace.Askpass {
	if f.locked || !f.stored[alias] {
		return nil
	}
	return &workspace.Askpass{Bin: "/bin/vantage", Socket: "/tmp/s"}
}

func (f *fakePasswords) Stored(alias string) askpass.State {
	switch {
	case f.locked:
		return askpass.Locked
	case f.stored[alias]:
		return askpass.Stored
	}
	return askpass.None
}

func lockedHosts(t *testing.T) (model, *fakePasswords) {
	t.Helper()
	m := openHosts(t, twoHosts())
	f := &fakePasswords{locked: true, pass: "pw", stored: map[string]bool{"web": true}}
	m.cfg.Passwords = f
	return m, f
}

func typeText(m model, s string) model {
	for _, r := range s {
		m, _ = press(m, string(r))
	}
	return m
}

func TestConnectAsksForTheVaultPassphraseFirst(t *testing.T) {
	m, f := lockedHosts(t)
	m, _ = press(m, "v") // the split needs the vault before it can know about a password
	if m.unlock == nil {
		t.Fatal("a locked vault must be asked before the split")
	}
	if got := popupText(m); !strings.Contains(got, "password vault") || strings.Contains(got, "pw") {
		t.Errorf("popup = %q", got)
	}
	m = typeText(m, "no")
	if got := popupText(m); strings.Contains(got, "no") || !strings.Contains(got, "••") {
		t.Errorf("the passphrase must show as dots only: %q", got)
	}
	m, cmd := press(m, "enter")
	m = settle(m, cmd)
	if m.unlock == nil || !strings.Contains(popupText(m), "wrong passphrase") {
		t.Fatalf("a wrong passphrase keeps the question open: %+v", m.unlock)
	}
	m = typeText(m, "pw")
	m, cmd = press(m, "enter")
	m = settle(m, cmd)
	if m.unlock != nil || f.locked {
		t.Errorf("unlocked: popup %+v, locked %v", m.unlock, f.locked)
	}
	if m.notice != "open a workspace first" {
		t.Errorf("the split must carry on after unlocking, notice %q", m.notice)
	}
	if !reflect.DeepEqual(f.tried, []string{"no", "pw"}) {
		t.Errorf("tried %v", f.tried)
	}
}

func TestEscSkipsTheVaultAndCarriesOn(t *testing.T) {
	m, f := lockedHosts(t)
	m, _ = press(m, "v", "esc")
	if m.unlock != nil || !f.locked {
		t.Fatalf("esc skips without unlocking: popup %+v, locked %v", m.unlock, f.locked)
	}
	if !strings.Contains(m.notice, "vault locked") && m.notice != "open a workspace first" {
		t.Errorf("notice %q", m.notice)
	}
}

func TestDetailsShowPasswordStateNeverTheValue(t *testing.T) {
	m, f := lockedHosts(t)
	if got := strings.Join(m.hostDetails(), "\n"); !strings.Contains(got, "locked") {
		t.Errorf("locked vault: %s", got)
	}
	f.locked = false
	got := strings.Join(m.hostDetails(), "\n")
	if !strings.Contains(got, "stored") || strings.Contains(got, f.pass) {
		t.Errorf("open vault: %s", got)
	}
}

func TestAskpassForNeedsAVaultAndAPassword(t *testing.T) {
	if askpassFor(nil, "web") != nil {
		t.Error("no vault, no helper")
	}
	f := &fakePasswords{stored: map[string]bool{"web": true}}
	if askpassFor(f, "web") == nil || askpassFor(f, "db") != nil {
		t.Error("the helper goes only to hosts with a stored password")
	}
}

func TestUnlockKeyOpensTheQuestion(t *testing.T) {
	m, _ := lockedHosts(t)
	m, _ = press(m, "u")
	if m.unlock == nil {
		t.Fatal("u opens the passphrase question")
	}
}

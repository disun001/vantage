package tui

import (
	"errors"
	"strings"
	"testing"
)

func formWithVault(t *testing.T, f *fakePasswords) (model, *fakeHosts) {
	t.Helper()
	fh := twoHosts()
	m := openHosts(t, fh)
	m.cfg.Passwords = f
	return m, fh
}

func TestPasswordLineWaitsForTheHost(t *testing.T) {
	m, _ := formWithVault(t, &fakePasswords{stored: map[string]bool{}})
	m, _ = press(m, "a")
	if got := popupText(m); !strings.Contains(got, "save the ssh config first") || !strings.Contains(got, "Apart") {
		t.Errorf("new host form: %q", got)
	}
	for i := 0; i < 8; i++ { // tab all the way round: the password line is not reachable yet
		m, _ = press(m, "tab")
		if m.form.focus >= len(hostFields) {
			t.Fatalf("focus %d reached the disabled password line", m.form.focus)
		}
	}
}

func TestSshConfigAndPasswordSaveApart(t *testing.T) {
	pw := &fakePasswords{stored: map[string]bool{}, pass: "v"}
	m, fh := formWithVault(t, pw)
	m, _ = press(m, "a")
	m = typeText(m, "new")
	m, cmd := press(m, "enter")
	if m.form == nil {
		t.Fatal("with a vault the form stays open after the ssh config saved")
	}
	m = settleAll(m, cmd)
	if m.form == nil || !m.form.edit || !strings.Contains(popupText(m), "✓ saved") {
		t.Fatalf("after the ssh config save: %+v", m.form)
	}
	if len(pw.stored) != 0 {
		t.Fatal("saving the ssh config must not touch the vault")
	}
	if got := fh.hosts[len(fh.hosts)-1].Alias; got != "new" {
		t.Errorf("ssh config got %q", got)
	}

	m, _ = press(m, "shift+tab") // wraps to the password line, now that the host exists
	if !m.form.onPassword() {
		t.Fatalf("focus %d is not the password line", m.form.focus)
	}
	m = typeText(m, "hunter2")
	if got := popupText(m); strings.Contains(got, "hunter2") || !strings.Contains(got, "•••••••") {
		t.Errorf("the password must show as dots only: %q", got)
	}
	callsBefore := len(fh.calls)
	m, cmd = press(m, "enter")
	m = settleAll(m, cmd)
	if !pw.stored["new"] || m.form.pw != "" || !strings.Contains(popupText(m), "✓ stored") {
		t.Errorf("password not saved: stored %v, form %+v", pw.stored, m.form)
	}
	if len(fh.calls) != callsBefore {
		t.Errorf("saving the password touched the ssh config: %v", fh.calls[callsBefore:])
	}
}

func TestFailedPasswordLeavesTheSshConfigAlone(t *testing.T) {
	pw := &fakePasswords{stored: map[string]bool{}, setErr: errors.New("disk full")}
	m, fh := formWithVault(t, pw)
	m, _ = press(m, "e", "shift+tab", "shift+tab") // the edit form starts on host: two back reach the password line
	callsBefore := len(fh.calls)
	m = typeText(m, "x")
	m, cmd := press(m, "enter")
	m = settleAll(m, cmd)
	if !strings.Contains(popupText(m), "✗ disk full") || len(fh.calls) != callsBefore {
		t.Errorf("popup %q, ssh calls %v", popupText(m), fh.calls)
	}
}

func TestPasswordSaveAsksForTheVaultAndEscSavesNothing(t *testing.T) {
	pw := &fakePasswords{locked: true, pass: "v", stored: map[string]bool{}}
	m, _ := formWithVault(t, pw)
	m, _ = press(m, "e", "shift+tab", "shift+tab") // the edit form starts on host: two back reach the password line
	m = typeText(m, "x")
	m, _ = press(m, "enter")
	if m.unlock == nil {
		t.Fatal("a locked vault must be opened first")
	}
	m, _ = press(m, "esc")
	if m.unlock != nil || len(pw.stored) != 0 || !strings.Contains(popupText(m), "not saved") {
		t.Errorf("esc: popup %q, stored %v", popupText(m), pw.stored)
	}
}

func TestFirstPasswordChoosesTheVaultPassphraseTwice(t *testing.T) {
	pw := &fakePasswords{missing: true, pass: "v1", stored: map[string]bool{}}
	m, _ := formWithVault(t, pw)
	m, _ = press(m, "e", "shift+tab", "shift+tab") // the edit form starts on host: two back reach the password line
	m = typeText(m, "x")
	m, _ = press(m, "enter")
	if m.unlock == nil || !m.unlock.create || !strings.Contains(popupText(m), "choose a passphrase") {
		t.Fatalf("a missing vault asks for a new passphrase: %+v", m.unlock)
	}
	m = typeText(m, "v1")
	m, _ = press(m, "enter")
	if !strings.Contains(popupText(m), "repeat it") {
		t.Fatalf("second entry: %q", popupText(m))
	}
	m = typeText(m, "other")
	m, _ = press(m, "enter")
	if m.unlock == nil || !strings.Contains(popupText(m), "differ") || len(pw.tried) != 0 {
		t.Fatalf("a mismatch starts again without creating anything: %q, tried %v", popupText(m), pw.tried)
	}
	m = typeText(m, "v1")
	m, _ = press(m, "enter")
	m = typeText(m, "v1")
	m, cmd := press(m, "enter")
	m = settleAll(m, cmd)
	if len(pw.tried) != 1 || pw.tried[0] != "v1" || !pw.stored["web"] {
		t.Errorf("tried %v, stored %v", pw.tried, pw.stored)
	}
}

func TestCtrlXRemovesTheStoredPassword(t *testing.T) {
	pw := &fakePasswords{stored: map[string]bool{"web": true}}
	m, _ := formWithVault(t, pw)
	m, _ = press(m, "e", "shift+tab", "shift+tab") // the edit form starts on host: two back reach the password line
	m, cmd := press(m, "ctrl+x")
	m = settleAll(m, cmd)
	if pw.stored["web"] || !strings.Contains(popupText(m), "✓ removed") {
		t.Errorf("stored %v, popup %q", pw.stored, popupText(m))
	}
}

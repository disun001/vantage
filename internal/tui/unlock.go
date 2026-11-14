package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/askpass"
	"github.com/disun001/vantage/internal/vault"
	"github.com/disun001/vantage/internal/workspace"
)

// Passwords is the stored-password vault of this run. askpass.Session
// implements it. Nil turns stored passwords off.
type Passwords interface {
	// NeedsUnlock reports a vault that still wants its passphrase.
	NeedsUnlock() bool
	// Unlock opens it, or returns vault.ErrWrongKey.
	Unlock(passphrase string) error
	// Askpass returns what a pane needs to answer the host's password prompt,
	// or nil when there is no stored password to use.
	Askpass(alias string) *workspace.Askpass
	// Stored says what the vault knows about the host.
	Stored(alias string) askpass.State
}

// unlockDoneMsg reports the end of an unlock attempt.
type unlockDoneMsg struct{ err error }

// unlockPrompt asks for the vault passphrase before an action that may need
// a stored password. then runs once the vault is open, or without it when the
// person skips with Esc.
type unlockPrompt struct {
	pass string
	err  string
	busy bool // scrypt is running; keys wait
	then func(model) (tea.Model, tea.Cmd)
}

// afterUnlock runs next now, or after the passphrase has been asked.
func (m model) afterUnlock(next func(model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	if p := m.cfg.Passwords; p != nil && p.NeedsUnlock() {
		m.unlock = &unlockPrompt{then: next}
		return m, nil
	}
	return next(m)
}

// onUnlockKey types the passphrase, tries it on Enter and skips on Esc.
func (m model) onUnlockKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := *m.unlock
	m.unlock = &u
	if u.busy {
		return m, nil
	}
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.unlock = nil
		m.notice = "vault locked: ssh will ask for the password"
		return u.then(m)
	case tea.KeyEnter:
		if u.pass == "" {
			return m, nil
		}
		u.busy, u.err = true, ""
		p, pass := m.cfg.Passwords, u.pass
		return m, func() tea.Msg { return unlockDoneMsg{err: p.Unlock(pass)} }
	case tea.KeyBackspace:
		if r := []rune(u.pass); len(r) > 0 {
			u.pass = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		u.pass = ""
	case tea.KeyRunes, tea.KeySpace:
		u.pass += string(k.Runes)
	}
	return m, nil
}

func (m model) onUnlockDone(msg unlockDoneMsg) (tea.Model, tea.Cmd) {
	if m.unlock == nil {
		return m, nil
	}
	u := *m.unlock
	if msg.err != nil {
		u.busy, u.pass = false, ""
		u.err = "wrong passphrase"
		if !errors.Is(msg.err, vault.ErrWrongKey) {
			u.err = oneLine(msg.err)
		}
		m.unlock = &u
		return m, nil
	}
	m.unlock = nil
	return u.then(m)
}

// unlockBox draws the passphrase question; the typed text shows as dots.
func (m model) unlockBox() []string {
	inner := max(10, min(44, m.w-6))
	rows := []string{popTitle("password vault"), "", dimStyle.Render("passphrase")}
	rows = append(rows, strings.Repeat("•", len([]rune(m.unlock.pass)))+cursorStyle.Render(" "))
	switch {
	case m.unlock.busy:
		rows = append(rows, "", dimStyle.Render("opening…"))
	case m.unlock.err != "":
		rows = append(rows, "", noticeStyle.Render(m.unlock.err))
	}
	rows = append(rows, "", dimStyle.Render("enter unlock · esc skip"))
	return framed(rows, inner)
}

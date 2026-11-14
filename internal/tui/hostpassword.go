package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pwIdx is the focus index of the password line, after the ssh config fields.
var pwIdx = len(hostFields)

// pwSavedMsg reports a finished password save or removal.
type pwSavedMsg struct {
	removed bool
	err     error
}

// count is how many lines the form can focus. The password line only exists
// once the host does, because the vault is keyed by its alias.
func (f hostForm) count() int {
	if f.pwOn && f.edit {
		return pwIdx + 1
	}
	return len(hostFields)
}

func (f hostForm) onPassword() bool { return f.pwOn && f.edit && f.focus == pwIdx }

// text is the string the focused line edits.
func (f *hostForm) text() *string {
	if f.onPassword() {
		return &f.pw
	}
	return &f.vals[f.focus]
}

// typed adds runes to the focused line. Only the password may hold blanks.
func (f *hostForm) typed(s string) {
	if f.onPassword() {
		f.pw += s
		return
	}
	f.vals[f.focus] += strings.Join(strings.Fields(s), "") // no blanks in the ssh fields
}

// onHostSavedInForm shows the outcome of an ssh config save inside the open
// form, which stays open so the password can be set next.
func (m model) onHostSavedInForm(msg hostSavedMsg) (tea.Model, tea.Cmd) {
	f := *m.form
	m.form = &f
	if msg.err != nil && !msg.renamed {
		f.sshMsg = "✗ " + oneLine(msg.err)
		return m, nil
	}
	f.edit, f.from, f.vals[0], f.focus = true, msg.alias, msg.alias, min(f.focus, pwIdx)
	f.sshMsg = "✓ saved"
	if msg.err != nil {
		f.sshMsg = "✗ " + oneLine(msg.err)
	}
	m.hostCur = msg.alias
	return m, m.loadHosts()
}

// submitPassword stores the typed password, after opening the vault.
func (m model) submitPassword() (tea.Model, tea.Cmd) {
	f := *m.form
	if f.pw == "" {
		f.pwMsg = "nothing typed (ctrl+x removes the stored one)"
		m.form = &f
		return m, nil
	}
	m.form = &f
	save := func(m model) (tea.Model, tea.Cmd) { return m.runPassword(false) }
	return m.afterVault(save, m.passwordSkipped, true)
}

// removePassword deletes the stored password of the host.
func (m model) removePassword() (tea.Model, tea.Cmd) {
	rm := func(m model) (tea.Model, tea.Cmd) { return m.runPassword(true) }
	return m.afterVault(rm, m.passwordSkipped, false)
}

// passwordSkipped is what Esc on the vault question does: nothing is saved.
func (m model) passwordSkipped(mm model) (tea.Model, tea.Cmd) {
	if mm.form != nil {
		f := *mm.form
		f.pwMsg = "not saved: the vault stayed closed"
		mm.form = &f
	}
	return mm, nil
}

// runPassword writes to the open vault in the background (scrypt is slow).
func (m model) runPassword(remove bool) (tea.Model, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	f := *m.form
	m.form = &f
	f.pwMsg = "saving…"
	pw, alias, typed := m.cfg.Passwords, f.from, f.pw
	return m, func() tea.Msg {
		if remove {
			return pwSavedMsg{removed: true, err: pw.Delete(alias)}
		}
		return pwSavedMsg{err: pw.Set(alias, typed)}
	}
}

func (m model) onPwSaved(msg pwSavedMsg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	f := *m.form
	m.form = &f
	switch {
	case msg.err != nil:
		f.pwMsg = "✗ " + oneLine(msg.err)
	case msg.removed:
		f.pwMsg, f.pw = "✓ removed", ""
	default:
		f.pwMsg, f.pw = "✓ stored, encrypted", ""
	}
	return m, nil
}

// passwordRows draws the password section under the ssh config fields. The
// two sections save apart, so a failure in one never half-saves the other.
func passwordRows(f hostForm, inner int) []string {
	rows := []string{dimStyle.Render(fit("ssh config", inner))}
	if f.sshMsg != "" {
		rows = append(rows, dimStyle.Render(f.sshMsg))
	}
	rows = append(rows, "")
	label := fit("password", 9)
	switch {
	case !f.edit:
		rows = append(rows, dimStyle.Render(label+"save the ssh config first"))
	case f.focus == pwIdx:
		rows = append(rows, curStyle.Render(label)+strings.Repeat("•", len([]rune(f.pw)))+cursorStyle.Render(" "))
	default:
		rows = append(rows, dimStyle.Render(label)+strings.Repeat("•", len([]rune(f.pw))))
	}
	if f.pwMsg != "" {
		rows = append(rows, dimStyle.Render(f.pwMsg))
	}
	rows = append(rows, "",
		dimStyle.Render("enter saves the line's section only: ssh config goes to ~/.ssh/config as plain text, the password to the encrypted vault. Apart, so one failing never half-saves the other."),
		dimStyle.Render("tab next · esc closes"))
	return rows
}

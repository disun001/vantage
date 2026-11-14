package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/tmux"
	"github.com/disun001/vantage/internal/workspace"
)

type (
	// hostOpenMsg carries the open panes connected to the host Enter was
	// pressed on.
	hostOpenMsg struct {
		alias string
		panes []tmux.PaneRef
		err   error
	}
	// hostGoMsg reports that session now has the pane or window to show.
	hostGoMsg struct {
		session string
		err     error
	}
)

// hostPick is the open choice between several panes connected to one host.
type hostPick struct {
	alias string
	panes []tmux.PaneRef
	cur   int
}

// updateHosts handles the messages of the hosts view.
func (m model) updateHosts(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case paneTitleMsg:
		return m.onPaneTitle(msg), nil
	case hostsMsg:
		return m.onHostsLoaded(msg)
	case pwSavedMsg:
		return m.onPwSaved(msg)
	case unlockDoneMsg:
		return m.onUnlockDone(msg)
	case hostInfoMsg:
		return m.onHostInfo(msg), nil
	case hostUsageMsg:
		return m.onHostUsage(msg), nil
	case hostSplitMsg:
		return m.onHostSplit(msg)
	case hostOpenMsg:
		return m.onHostOpen(msg)
	case hostGoMsg:
		return m.onHostGo(msg)
	case hostShutMsg:
		return m.onHostShut(msg), nil
	case hostClosedMsg:
		return m.onHostClosed(msg)
	case hostSavedMsg:
		return m.onHostSaved(msg)
	}
	return m, nil
}

// clientTTY is tty when known, else the tty of the embedded client, which
// tmux only reports after it has registered.
func clientTTY(c tmux.Client, tty string, pid int) (string, error) {
	if tty != "" {
		return tty, nil
	}
	tty, err := c.ClientTTY(pid)
	if err != nil {
		return "", errors.New("tmux client not ready, try again")
	}
	return tty, nil
}

// openHost goes to the pane already connected to the host under the cursor.
// With several it asks which; with none it opens ssh in a new window.
func (m model) openHost() (tea.Model, tea.Cmd) {
	if len(m.hosts) == 0 {
		return m, nil
	}
	alias := m.hosts[m.hostPos()].Alias
	m.notice = ""
	u := m.cfg.HostUse
	if u == nil {
		return m.newHostWindow(alias)
	}
	return m, func() tea.Msg {
		panes, err := u.Live(alias)
		return hostOpenMsg{alias: alias, panes: panes, err: err}
	}
}

func (m model) onHostOpen(msg hostOpenMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		m.notice = "could not look for open panes: " + oneLine(msg.err)
		return m, nil
	case len(msg.panes) == 0:
		return m.newHostWindow(msg.alias)
	case len(msg.panes) == 1:
		return m.goPane(msg.panes[0])
	}
	m.hostPick = &hostPick{alias: msg.alias, panes: msg.panes}
	return m, nil
}

// goPane focuses the pane in tmux, then shows its session.
func (m model) goPane(p tmux.PaneRef) (tea.Model, tea.Cmd) {
	c := m.cfg.Client
	return m, func() tea.Msg { return hostGoMsg{session: p.Session, err: c.FocusPane(p.Pane)} }
}

// newHostWindow opens ssh to the alias in a new window of the session the
// embedded client shows.
func (m model) newHostWindow(alias string) (tea.Model, tea.Cmd) {
	return m.afterUnlock(func(m model) (tea.Model, tea.Cmd) { return m.newHostWindowNow(alias) })
}

func (m model) newHostWindowNow(alias string) (tea.Model, tea.Cmd) {
	if m.term == nil {
		m.notice = "open a workspace first"
		return m, nil
	}
	c, tty, pid, pw := m.cfg.Client, m.tty, m.term.PID(), m.cfg.Passwords
	return m, func() tea.Msg {
		if !workspace.ValidSSHAlias(alias) {
			return hostGoMsg{err: fmt.Errorf("invalid ssh alias %q", alias)}
		}
		tty, err := clientTTY(c, tty, pid)
		if err != nil {
			return hostGoMsg{err: err}
		}
		session, err := c.ClientSession(tty)
		if err == nil {
			_, err = c.NewWindowRun(session, alias, workspace.SSHCommandWith(alias, askpassFor(pw, alias)))
		}
		return hostGoMsg{session: session, err: err}
	}
}

// onHostGo shows the session and moves the keys to it, or says what failed.
func (m model) onHostGo(msg hostGoMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = oneLine(msg.err)
		return m, nil
	}
	m = m.show(msg.session)
	m.view = viewSessions
	if m.term != nil {
		m.focus = focusTerm
	}
	return m, m.refresh()
}

// askpassFor is the helper settings for alias, or nil without a vault.
func askpassFor(pw Passwords, alias string) *workspace.Askpass {
	if pw == nil {
		return nil
	}
	return pw.Askpass(alias)
}

func oneLine(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }

// onHostPickKey moves in the choice, picks with Enter or a digit, and drops
// it with Esc.
func (m model) onHostPickKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := *m.hostPick
	m.hostPick = &p
	n := len(p.panes)
	switch s := k.String(); s {
	case "esc", "ctrl+c", "q":
		m.hostPick = nil
	case "j", "down", "tab":
		p.cur = (p.cur + 1) % n
	case "k", "up", "shift+tab":
		p.cur = (p.cur + n - 1) % n
	case "enter":
		m.hostPick = nil
		return m.goPane(p.panes[p.cur])
	default:
		if len(s) == 1 && s[0] >= '1' && int(s[0]-'1') < min(n, 9) {
			m.hostPick = nil
			return m.goPane(p.panes[s[0]-'1'])
		}
	}
	return m, nil
}

const pickRows = 12 // panes shown at once

// box draws the choice as a pop-up: one line per pane, the chosen one marked.
func (p hostPick) box(w int) []string {
	inner := max(10, min(44, w-6))
	rows := []string{popTitle("connected to " + p.alias), ""}
	start := max(0, min(p.cur-pickRows/2, len(p.panes)-pickRows))
	for i := start; i < min(len(p.panes), start+pickRows); i++ {
		r := p.panes[i]
		num := " "
		if i < 9 {
			num = fmt.Sprint(i + 1)
		}
		text := fmt.Sprintf("%s %s:%d %s", num, r.Session, r.Window, r.WindowName)
		if i == p.cur {
			rows = append(rows, selStyle.Render(fit("▸"+text, inner)))
		} else {
			rows = append(rows, fit(" "+text, inner))
		}
	}
	rows = append(rows, "", dimStyle.Render("enter go · 1-9 · esc cancels"))
	return framed(rows, inner)
}

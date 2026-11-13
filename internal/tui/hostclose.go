package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/hostuse"
	"github.com/disun001/vantage/internal/tmux"
)

type (
	// hostShutMsg carries the open panes connected to the host `x` was
	// pressed on.
	hostShutMsg struct {
		alias string
		panes []tmux.PaneRef
		err   error
	}
	// hostClosedMsg reports a finished close of a host's panes.
	hostClosedMsg struct {
		alias string
		n     int
		err   error
	}
)

// hostShut is the open question about closing every pane of one host.
type hostShut struct {
	alias string
	panes []tmux.PaneRef
}

// shutHost looks up the open panes of the host under the cursor, then asks.
func (m model) shutHost() (tea.Model, tea.Cmd) {
	u := m.cfg.HostUse
	if len(m.hosts) == 0 || u == nil {
		return m, nil
	}
	alias := m.hosts[m.hostPos()].Alias
	m.notice = ""
	return m, func() tea.Msg {
		panes, err := u.Live(alias)
		return hostShutMsg{alias: alias, panes: panes, err: err}
	}
}

func (m model) onHostShut(msg hostShutMsg) model {
	switch {
	case msg.err != nil:
		m.notice = "could not look for open panes: " + oneLine(msg.err)
	case len(msg.panes) == 0:
		m.notice = "no open panes for " + msg.alias
	default:
		m.hostShut = &hostShut{alias: msg.alias, panes: msg.panes}
	}
	return m
}

// onHostShutKey answers the question: y closes, anything else cancels.
func (m model) onHostShutKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := *m.hostShut
	m.hostShut, m.notice = nil, ""
	if k.String() != "y" {
		return m, nil
	}
	u := m.cfg.HostUse
	return m, func() tea.Msg {
		// Look again: panes may have come or gone since the question.
		panes, err := u.Live(s.alias)
		if err != nil {
			return hostClosedMsg{alias: s.alias, err: err}
		}
		err = u.Close(hostuse.Usage{Panes: panes})
		return hostClosedMsg{alias: s.alias, n: len(panes), err: err}
	}
}

func (m model) onHostClosed(msg hostClosedMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		m.notice = oneLine(fmt.Errorf("closing panes of %s failed: %w", msg.alias, msg.err))
	case msg.n == 0:
		m.notice = "no open panes for " + msg.alias
	default:
		m.notice = fmt.Sprintf("closed %d pane(s) of %s", msg.n, msg.alias)
	}
	return m, m.refresh() // closed panes change the session list
}

// box draws the question: the panes that go, and what else may go with them.
func (s hostShut) box(w int) []string {
	inner := max(10, min(44, w-6))
	rows := []string{dangerTitle("close panes of " + s.alias), ""}
	for i, r := range s.panes {
		if i == pickRows {
			rows = append(rows, fmt.Sprintf("  … %d more", len(s.panes)-pickRows))
			break
		}
		rows = append(rows, fit(fmt.Sprintf("  %s:%d %s", r.Session, r.Window, r.WindowName), inner))
	}
	rows = append(rows, "",
		noticeStyle.Render(fmt.Sprintf("%d open pane(s) will be closed.", len(s.panes))),
		dimStyle.Render("A window or session left empty closes too."),
		"", dimStyle.Render("y close · any other key cancels"))
	return framed(rows, inner)
}

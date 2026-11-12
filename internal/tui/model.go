package tui

import (
	"fmt"
	"os/exec"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/embed"
	"github.com/disun001/vantage/internal/tmux"
)

const (
	sidebarW  = 26 // columns, divider not included
	tickEvery = 2 * time.Second
)

// Config wires the TUI to the outside world. The funcs keep this package
// free of workspace file access and make it testable.
type Config struct {
	Client tmux.Client
	// Saved lists saved workspace names.
	Saved func() ([]string, error)
	// Restore creates the session for a saved workspace if it is not live.
	Restore func(name string) error
}

type focus int

const (
	focusSidebar focus = iota
	focusTerm
)

// hub carries what goroutines need to reach the running program. It is a
// pointer so every copy of the model shares it.
type hub struct {
	send  func(tea.Msg)
	dirty atomic.Bool
}

type model struct {
	cfg     Config
	hub     *hub
	w, h    int
	rows    []row
	cursor  int
	focus   focus
	term    *embed.Term
	tty     string // tty of the embedded tmux client, once known
	current string // session the embedded client shows
	notice  string
	shown   bool // a live session was auto-shown once; do not re-attach after a detach
}

type (
	outputMsg struct{}
	tickMsg   struct{}
	rowsMsg   struct {
		rows []row
		err  error
	}
	exitMsg  struct{ pid int }
	readyMsg struct { // a session is live and can be shown
		name string
		err  error
	}
	edgeMsg struct {
		atLeft bool
		err    error
	}
)

func newModel(cfg Config, h *hub) model { return model{cfg: cfg, hub: h} }

func (m model) Init() tea.Cmd { return tea.Batch(m.refresh(), tick()) }

func tick() tea.Cmd {
	return tea.Tick(tickEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		live, err := m.cfg.Client.ListSessions()
		if err != nil {
			return rowsMsg{err: err}
		}
		panes, _ := m.cfg.Client.ListAllPanes() // counts are cosmetic
		var saved []string
		if m.cfg.Saved != nil {
			saved, _ = m.cfg.Saved()
		}
		return rowsMsg{rows: mergeRows(live, panes, saved)}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.term != nil {
			m.term.Resize(m.termW(), m.h)
		}
	case outputMsg:
		m.hub.dirty.Store(false)
	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())
	case rowsMsg:
		return m.onRows(msg), nil
	case exitMsg:
		if m.term != nil && m.term.PID() == msg.pid {
			m.term, m.tty, m.current = nil, "", ""
			m.focus = focusSidebar
		}
		return m, m.refresh()
	case readyMsg:
		return m.onReady(msg)
	case edgeMsg:
		return m.onEdge(msg), nil
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m model) onRows(msg rowsMsg) model {
	if msg.err != nil {
		m.notice = msg.err.Error()
		return m
	}
	m.rows = msg.rows
	if m.cursor >= len(m.rows) {
		m.cursor = max(0, len(m.rows)-1)
	}
	// First sight of live sessions: show one so the right side is not empty.
	if !m.shown {
		for _, r := range m.rows {
			if r.Live {
				m.shown = true
				return m.show(r.Name)
			}
		}
	}
	return m
}

func (m model) onReady(msg readyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = msg.err.Error()
		return m, nil
	}
	m.notice = ""
	m = m.show(msg.name)
	if m.term != nil {
		m.focus = focusTerm
	}
	return m, m.refresh()
}

// show makes the embedded client display a live session, attaching a client
// first if none is running.
func (m model) show(name string) model {
	if m.term == nil {
		t, err := m.attach(name)
		if err != nil {
			m.notice = err.Error()
			return m
		}
		m.term = t
		m.current = name
		return m
	}
	m.current = name
	// switch-client needs the tty, which tmux only knows after attach.
	if m.tty == "" {
		m.tty, _ = m.cfg.Client.ClientTTY(m.term.PID())
	}
	if m.tty == "" {
		m.notice = "tmux client not ready, try again"
		return m
	}
	if err := m.cfg.Client.SwitchClient(m.tty, name); err != nil {
		m.notice = err.Error()
	}
	return m
}

func (m model) attach(name string) (*embed.Term, error) {
	cmd := m.cfg.Client.AttachCmd(name)
	return m.startTerm(cmd)
}

func (m model) startTerm(cmd *exec.Cmd) (*embed.Term, error) {
	cols, rows := m.termW(), m.h
	if cols < 1 || rows < 1 {
		return nil, fmt.Errorf("window too small")
	}
	h := m.hub
	return embed.Start(cmd, cols, rows,
		func() {
			if h.dirty.CompareAndSwap(false, true) {
				h.send(outputMsg{})
			}
		},
		func() { h.send(exitMsg{pid: cmd.Process.Pid}) },
	)
}

func (m model) termW() int { return max(1, m.w-sidebarW-1) }

func (m model) onEdge(msg edgeMsg) model {
	if msg.err != nil || msg.atLeft {
		m.focus = focusSidebar
		return m
	}
	if m.term != nil {
		_ = m.term.Write([]byte("\x1bh"))
	}
	return m
}

func (m model) edgeCmd() tea.Cmd {
	c, tty, pid := m.cfg.Client, m.tty, 0
	if m.term != nil {
		pid = m.term.PID()
	}
	return func() tea.Msg {
		if tty == "" {
			var err error
			if tty, err = c.ClientTTY(pid); err != nil {
				return edgeMsg{err: err}
			}
		}
		at, err := c.PaneAtLeft(tty)
		return edgeMsg{atLeft: at, err: err}
	}
}

func (m model) selectCmd() tea.Cmd {
	if m.cursor >= len(m.rows) {
		return nil
	}
	r, cfg := m.rows[m.cursor], m.cfg
	return func() tea.Msg {
		if !r.Live && cfg.Restore != nil {
			if err := cfg.Restore(r.Name); err != nil {
				return readyMsg{name: r.Name, err: err}
			}
		}
		return readyMsg{name: r.Name}
	}
}

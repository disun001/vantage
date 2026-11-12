package tui

import (
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/embed"
	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/tmux"
)

const (
	sidebarW  = 26 // columns, divider not included
	tickEvery = 2 * time.Second
	warmEvery = 150 * time.Millisecond // polling while a new client registers
)

// Config wires the TUI to the outside world. The funcs keep this package
// free of workspace file access and make it testable.
type Config struct {
	Client tmux.Client
	// Saved lists saved workspace names.
	Saved func() ([]string, error)
	// Restore creates the session for a saved workspace if it is not live.
	Restore func(name string) error
	// Manage creates, renames and deletes workspaces. Nil turns the n, R
	// and d keys off.
	Manage Manager
	// SSH lists, adds and edits ssh hosts. Nil turns the hosts view off.
	SSH HostStore
	// HostUse warns before a host that workspaces or open panes use is
	// deleted, and closes those panes. Nil skips the check.
	HostUse HostUsage
	// KeyLog, when set, gets one line per non-text key press (see logKey).
	KeyLog io.Writer
}

// Manager changes workspaces on behalf of the sidebar.
type Manager interface {
	// Create starts and saves a new workspace.
	Create(name string) error
	// Rename renames the saved workspace and the live session.
	Rename(from, to string) error
	// Delete removes the saved file and kills the live session.
	Delete(name string) error
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
	cfg      Config
	hub      *hub
	w, h     int
	rows     []row
	cur      key             // the line under the cursor, by name
	top      int             // first visible item, for scrolling
	open     map[string]bool // sessions the user opened or closed by hand
	focus    focus
	term     *embed.Term
	tty      string // tty of the embedded tmux client, once known
	bar      statusBar
	current  string // session the embedded client shows
	notice   string
	prompt   *prompt
	view     sidebarView
	hosts    []sshconfig.Host
	hostCur  string // alias under the cursor in the hosts view
	hostTop  int
	form     *hostForm
	hostDel  *hostDelete // the delete question, while it is open
	hostPick *hostPick   // the choice of connected pane, while it is open
	shown    bool        // a live session was auto-shown once; do not re-attach after a detach
}

// statusBar is the tmux status bar of the embedded client. The embedded
// screen is made that many lines taller and the bar is cropped from the view,
// because the sidebar already lists the windows.
type statusBar struct {
	lines int
	top   bool
}

type (
	outputMsg struct{}
	tickMsg   struct{}
	warmMsg   struct{} // a client was just started: look for its tty soon
	rowsMsg   struct {
		rows    []row
		err     error
		pid     int // client the tty and bar belong to
		tty     string
		bar     statusBar
		haveBar bool
	}
	manageMsg struct { // a workspace create, rename or delete finished
		op       promptKind
		from, to string
		err      error
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

func warm() tea.Cmd {
	return tea.Tick(warmEvery, func(time.Time) tea.Msg { return warmMsg{} })
}

func (m model) refresh() tea.Cmd {
	c, tty, pid := m.cfg.Client, m.tty, 0
	if m.term != nil {
		pid = m.term.PID()
	}
	saved := m.cfg.Saved
	return func() tea.Msg {
		live, err := c.ListSessions()
		if err != nil {
			return rowsMsg{err: err}
		}
		wins, _ := c.ListWindows() // windows and counts are cosmetic
		panes, _ := c.ListAllPanes()
		var names []string
		if saved != nil {
			names, _ = saved()
		}
		msg := rowsMsg{rows: mergeRows(live, wins, panes, names), pid: pid}
		if tty == "" && pid != 0 {
			tty, _ = c.ClientTTY(pid)
		}
		msg.tty = tty
		if tty != "" {
			if n, top, err := c.StatusBar(tty); err == nil {
				msg.bar, msg.haveBar = statusBar{n, top}, true
			}
		}
		return msg
	}
}

// Update runs the message, then keeps the cursor on screen and starts the
// short poll that finds a newly attached client.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prev := m.term
	next, cmd := m.update(msg)
	nm, ok := next.(model)
	if !ok {
		return next, cmd
	}
	nm = nm.scrolled()
	if nm.term != nil && nm.term != prev {
		cmd = tea.Batch(cmd, warm())
	}
	return nm, cmd
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.term != nil {
			m.term.Resize(m.termW(), m.termH())
		}
	case outputMsg:
		m.hub.dirty.Store(false)
	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())
	case warmMsg:
		if m.term != nil && m.tty == "" {
			return m, tea.Batch(m.refresh(), warm())
		}
	case rowsMsg:
		return m.onRows(msg), nil
	case exitMsg:
		if m.term != nil && m.term.PID() == msg.pid {
			m.term, m.tty, m.current, m.bar = nil, "", "", statusBar{}
			m.focus = focusSidebar
			m.notice = "detached: enter reopens"
		}
		return m, m.refresh()
	case readyMsg:
		return m.onReady(msg)
	case manageMsg:
		return m.onManage(msg)
	case edgeMsg:
		return m.onEdge(msg), nil
	case tea.KeyMsg:
		return m.onKey(msg)
	case tea.MouseMsg:
		return m.onMouse(msg)
	}
	return m.updateHosts(msg)
}

func (m model) onRows(msg rowsMsg) model {
	if msg.err != nil {
		m.notice = msg.err.Error()
		return m
	}
	m.rows = msg.rows
	if m.term != nil && m.term.PID() == msg.pid {
		if msg.tty != "" {
			m.tty = msg.tty
		}
		if msg.haveBar && msg.bar != m.bar {
			m.bar = msg.bar
			m.term.Resize(m.termW(), m.termH())
		}
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
	cols, rows := m.termW(), m.termH()
	if cols < 1 || rows < 1 {
		return nil, fmt.Errorf("window too small")
	}
	h := m.hub
	return embed.Start(
		cmd, cols, rows,
		func() {
			if h.dirty.CompareAndSwap(false, true) {
				h.send(outputMsg{})
			}
		},
		func() { h.send(exitMsg{pid: cmd.Process.Pid}) },
	)
}

func (m model) termW() int { return max(1, m.w-sidebarW-1) }

// termH is the height of the embedded screen: the window plus the status bar
// that view() crops away.
func (m model) termH() int { return m.h + m.bar.lines }

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

// selectCmd opens the line under the cursor: it restores a saved workspace,
// and for a window line also makes that window the session's current one.
func (m model) selectCmd() tea.Cmd {
	it, ok := m.selected()
	if !ok {
		return nil
	}
	r, cfg, window := m.rows[it.Row], m.cfg, -1
	if it.Win >= 0 {
		window = r.Wins[it.Win].Index
	}
	return func() tea.Msg {
		if !r.Live && cfg.Restore != nil {
			if err := cfg.Restore(r.Name); err != nil {
				return readyMsg{name: r.Name, err: err}
			}
		}
		if window >= 0 {
			if err := cfg.Client.SelectWindow(r.Name, window); err != nil {
				return readyMsg{name: r.Name, err: err}
			}
		}
		return readyMsg{name: r.Name}
	}
}

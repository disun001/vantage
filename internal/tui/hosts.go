package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/hostuse"
	"github.com/disun001/vantage/internal/pick"
	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/tmux"
)

// HostStore reads and changes the ssh hosts the sidebar's hosts view shows.
// sshconfig.File implements it.
type HostStore interface {
	// List returns the concrete hosts in file order.
	List() ([]sshconfig.Host, error)
	// Own returns the options the host's own block sets: what an edit starts from.
	Own(alias string) (sshconfig.Host, error)
	// Add appends a new Host block.
	Add(sshconfig.Host) error
	// Edit changes the options of the block named from, and renames it when
	// the host's alias differs.
	Edit(from string, h sshconfig.Host) error
	// Delete removes a Host block.
	Delete(alias string) error
}

// HostUsage finds what depends on a host before it is deleted, closes the
// live panes connected to it and removes its panes from saved workspaces. hostuse.Finder implements it.
type HostUsage interface {
	// Find reports the workspaces and open panes that use the alias. Partial
	// results may come back with an error.
	Find(alias string) (hostuse.Usage, error)
	// Close kills the open panes of the usage.
	Close(hostuse.Usage) error
	// Strip removes the alias's panes from the saved workspaces.
	Strip(alias string) error
	// Live returns the open panes running ssh to the alias.
	Live(alias string) ([]tmux.PaneRef, error)
	// Rename points the saved workspaces' panes and the open panes for from
	// at the alias to.
	Rename(from, to string) error
}

// sidebarView is what the sidebar lists.
type sidebarView int

const (
	viewSessions sidebarView = iota
	viewHosts
)

type (
	// hostsMsg carries a fresh host list.
	hostsMsg struct {
		hosts []sshconfig.Host
		err   error
	}
	// hostUsageMsg carries what depends on a host that is about to go.
	hostUsageMsg struct {
		alias string
		use   hostuse.Usage
		err   error
	}
	// hostSplitMsg reports a finished ssh split.
	hostSplitMsg struct {
		alias string
		err   error
	}
	// hostSavedMsg reports a finished add, edit, rename or delete. alias is
	// the host's name afterwards (the old one for a delete).
	hostSavedMsg struct {
		alias   string
		deleted bool
		renamed bool // the alias changed, even if a later step failed
		err     error
	}
)

// hostField is one line of the add / edit form.
type hostField struct {
	label string
	ptr   func(*sshconfig.Host) *string
}

// hostFields are the form lines in order.
var hostFields = []hostField{
	{"alias", func(h *sshconfig.Host) *string { return &h.Alias }},
	{"host", func(h *sshconfig.Host) *string { return &h.HostName }},
	{"user", func(h *sshconfig.Host) *string { return &h.User }},
	{"port", func(h *sshconfig.Host) *string { return &h.Port }},
	{"jump", func(h *sshconfig.Host) *string { return &h.ProxyJump }},
}

// hostForm is the pop-up for adding or editing a host: every field is on
// screen, Tab moves between them, Enter saves, Esc drops everything.
type hostForm struct {
	edit  bool
	from  string    // the alias being edited, to tell a rename
	vals  [5]string // indexed like hostFields
	focus int
	err   string // why the last Enter did not save
}

// host collects the typed values.
func (f hostForm) host() sshconfig.Host {
	var h sshconfig.Host
	for i, fd := range hostFields {
		*fd.ptr(&h) = strings.TrimSpace(f.vals[i])
	}
	return h
}

// check finds the first field ssh would not accept, so the cursor can go
// there with a message.
func (f hostForm) check() (int, string) {
	h := f.host()
	if h.Alias == "" {
		return 0, "alias is required"
	}
	for i := range hostFields {
		only := sshconfig.Host{Alias: h.Alias}
		*hostFields[i].ptr(&only) = *hostFields[i].ptr(&h)
		if err := sshconfig.Validate(only); err != nil {
			return i, err.Error()
		}
	}
	return -1, ""
}

func (m model) hostPos() int {
	for i, h := range m.hosts {
		if h.Alias == m.hostCur {
			return i
		}
	}
	return 0
}

func (m model) moveHost(delta int) model {
	if len(m.hosts) == 0 {
		return m
	}
	m.hostCur = m.hosts[max(0, min(m.hostPos()+delta, len(m.hosts)-1))].Alias
	return m
}

func (m model) hostScrolled() model {
	c, p := m.listCap(), m.hostPos()
	if p < m.hostTop {
		m.hostTop = p
	}
	if p >= m.hostTop+c {
		m.hostTop = p - c + 1
	}
	m.hostTop = max(0, min(m.hostTop, len(m.hosts)-c))
	return m
}

// loadHosts reads the host list in the background.
func (m model) loadHosts() tea.Cmd {
	s := m.cfg.SSH
	return func() tea.Msg {
		hosts, err := s.List()
		return hostsMsg{hosts: hosts, err: err}
	}
}

func (m model) onHosts(msg hostsMsg) model {
	if msg.err != nil {
		m.notice = ansi.Strip(msg.err.Error())
		return m
	}
	m.hosts = msg.hosts
	if m.hostCur == "" || !m.hasHost(m.hostCur) {
		m.hostCur = ""
		if len(m.hosts) > 0 {
			m.hostCur = m.hosts[0].Alias
		}
	}
	return m
}

// neighbour is the host to park the cursor on when alias goes away: the one
// below it, else the one above.
func (m model) neighbour(alias string) string {
	for i, h := range m.hosts {
		if h.Alias != alias {
			continue
		}
		switch {
		case i+1 < len(m.hosts):
			return m.hosts[i+1].Alias
		case i > 0:
			return m.hosts[i-1].Alias
		}
	}
	return ""
}

func (m model) hasHost(alias string) bool {
	for _, h := range m.hosts {
		if h.Alias == alias {
			return true
		}
	}
	return false
}

// toggleHosts switches the sidebar between sessions and ssh hosts.
func (m model) toggleHosts() (model, tea.Cmd) {
	if m.cfg.SSH == nil {
		return m, nil
	}
	m.notice = ""
	if m.view == viewHosts {
		m.view = viewSessions
		return m, nil
	}
	m.view = viewHosts
	return m, m.loadHosts()
}

// onHostKey handles a sidebar key in the hosts view.
func (m model) onHostKey(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "ctrl+c", "q":
		if m.term != nil {
			m.term.Close()
		}
		return m, tea.Quit
	case "j", "down":
		m = m.moveHost(1)
	case "k", "up":
		m = m.moveHost(-1)
	case "g", "home":
		m = m.moveHost(-len(m.hosts))
	case "G", "end":
		m = m.moveHost(len(m.hosts))
	case "r":
		return m, m.loadHosts()
	case "a":
		m.form, m.notice = &hostForm{}, ""
	case "e":
		return m.startEdit(), nil
	case "d":
		if len(m.hosts) > 0 {
			return m.startDelete(m.hosts[m.hostPos()].Alias)
		}
	case "enter":
		return m.openHost()
	case "x":
		return m.shutHost()
	case "v":
		return m.splitHost("h") // side by side, like vim's :vsplit
	case "s":
		return m.splitHost("v") // stacked, like :split
	case "H", "esc", "h", "left":
		return m.toggleHosts()
	case "tab", "alt+l":
		if m.term != nil {
			m.focus = focusTerm
		}
	}
	return m, nil
}

// splitHost splits the embedded client's active pane and runs ssh to the
// selected host in the new one. dir is "h" (side by side) or "v" (stacked).
func (m model) splitHost(dir string) (tea.Model, tea.Cmd) {
	if len(m.hosts) == 0 {
		return m, nil
	}
	if m.term == nil {
		m.notice = "open a workspace first"
		return m, nil
	}
	alias, c, tty, pid := m.hosts[m.hostPos()].Alias, m.cfg.Client, m.tty, m.term.PID()
	m.notice = ""
	return m, func() tea.Msg {
		tty, err := clientTTY(c, tty, pid)
		if err != nil {
			return hostSplitMsg{alias: alias, err: err}
		}
		pane, cwd, err := c.ActivePane(tty)
		if err == nil {
			_, err = pick.Split(c, pane, dir, cwd, pick.Choice{Alias: alias})
		}
		return hostSplitMsg{alias: alias, err: err}
	}
}

// onHostSplit shows a failure, or moves the keys to the new pane.
func (m model) onHostSplit(msg hostSplitMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = msg.err.Error()
		return m, nil
	}
	m.focus = focusTerm
	return m, m.refresh()
}

// startDelete looks up what depends on the host, then asks. Without a usage
// finder it asks straight away.
func (m model) startDelete(alias string) (tea.Model, tea.Cmd) {
	m.notice = ""
	u := m.cfg.HostUse
	if u == nil {
		m.hostDel = &hostDelete{alias: alias}
		return m, nil
	}
	return m, func() tea.Msg {
		use, err := u.Find(alias)
		return hostUsageMsg{alias: alias, use: use, err: err}
	}
}

// onHostUsage opens the delete pop-up, with a warning when something uses
// the host.
func (m model) onHostUsage(msg hostUsageMsg) model {
	m.hostDel = &hostDelete{alias: msg.alias, use: msg.use, err: msg.err}
	return m
}

// onHostDelKey answers the delete question: y deletes, anything else cancels.
func (m model) onHostDelKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	alias := m.hostDel.alias
	m.hostDel, m.notice = nil, ""
	if k.String() != "y" {
		return m, nil
	}
	s, u := m.cfg.SSH, m.cfg.HostUse
	return m, func() tea.Msg {
		if err := s.Delete(alias); err != nil {
			return hostSavedMsg{alias: alias, deleted: true, err: err}
		}
		var errs []error
		if u != nil {
			// Look again: panes may have come or gone since the question.
			use, _ := u.Find(alias)
			if err := u.Close(use); err != nil {
				errs = append(errs, fmt.Errorf("closing its panes failed: %w", err))
			}
			if err := u.Strip(alias); err != nil {
				errs = append(errs, fmt.Errorf("workspaces not fully updated: %w", err))
			}
		}
		if err := errors.Join(errs...); err != nil {
			err = fmt.Errorf("host removed, but %w", err)
			return hostSavedMsg{alias: alias, deleted: true, err: err}
		}
		return hostSavedMsg{alias: alias, deleted: true}
	}
}

// startEdit opens the form on the host under the cursor, prefilled with what
// its own block sets, or says why it cannot be edited.
func (m model) startEdit() model {
	if len(m.hosts) == 0 {
		return m
	}
	alias := m.hosts[m.hostPos()].Alias
	own, err := m.cfg.SSH.Own(alias)
	if err != nil {
		m.notice = ansi.Strip(alias + ": " + err.Error())
		return m
	}
	f := hostForm{edit: true, from: alias, focus: 1}
	for i, fd := range hostFields {
		f.vals[i] = *fd.ptr(&own)
	}
	m.form, m.notice = &f, ""
	return m
}

// onFormKey feeds a key to the open form.
func (m model) onFormKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := *m.form
	m.form = &f
	n := len(hostFields)
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.form = nil
	case tea.KeyEnter:
		return m.submitForm()
	case tea.KeyTab, tea.KeyDown:
		f.focus = (f.focus + 1) % n
	case tea.KeyShiftTab, tea.KeyUp:
		f.focus = (f.focus + n - 1) % n
	case tea.KeyBackspace:
		if r := []rune(f.vals[f.focus]); len(r) > 0 {
			f.vals[f.focus] = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		f.vals[f.focus] = ""
	case tea.KeyRunes:
		f.vals[f.focus] += strings.Join(strings.Fields(string(k.Runes)), "") // no blanks in any field
	}
	return m, nil
}

// submitForm saves, or stops on the field that is wrong.
func (m model) submitForm() (tea.Model, tea.Cmd) {
	f := *m.form
	if i, msg := f.check(); i >= 0 {
		f.focus, f.err = i, ansi.Strip(msg)
		m.form = &f
		return m, nil
	}
	m.form, m.notice = nil, ""
	return m, m.saveHost(f)
}

// saveHost writes the finished form in the background.
func (m model) saveHost(f hostForm) tea.Cmd {
	s, u, h := m.cfg.SSH, m.cfg.HostUse, f.host()
	return func() tea.Msg {
		if !f.edit {
			return hostSavedMsg{alias: h.Alias, err: s.Add(h)}
		}
		if err := s.Edit(f.from, h); err != nil {
			return hostSavedMsg{alias: f.from, err: err}
		}
		msg := hostSavedMsg{alias: h.Alias, renamed: f.from != h.Alias}
		if msg.renamed && u != nil {
			if err := u.Rename(f.from, h.Alias); err != nil {
				msg.err = fmt.Errorf("host renamed, but %w", err)
			}
		}
		return msg
	}
}

func (m model) onHostSaved(msg hostSavedMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		m.notice = ansi.Strip(fmt.Sprintf("%s: %v", msg.alias, msg.err))
		if msg.renamed {
			m.hostCur = msg.alias
		}
	case msg.deleted:
		m.notice, m.hostCur = "", m.neighbour(msg.alias)
	default:
		m.notice, m.hostCur = "", msg.alias
	}
	if msg.deleted {
		return m, tea.Batch(m.loadHosts(), m.refresh()) // closed panes change the session list
	}
	return m, m.loadHosts()
}

// hostsSidebar draws the hosts view.
func (m model) hostsSidebar() []string {
	out := make([]string, m.h)
	for i := range out {
		out[i] = strings.Repeat(" ", sidebarW)
	}
	title := "ssh hosts"
	if m.focus == focusSidebar {
		title = "▌ssh hosts"
	}
	out[0] = fit(titleStyle.Render(title), sidebarW)
	note := m.noticeLines()
	cur := m.hostPos()
	y := firstRowY
	for i := m.hostTop; i < len(m.hosts); i++ {
		if y >= m.h-1-len(note) {
			break
		}
		out[y] = m.highlight(hostLine(m.hosts[i]), i == cur)
		y++
	}
	if len(m.hosts) == 0 {
		out[firstRowY] = fit(dimStyle.Render("no hosts: a adds one"), sidebarW)
	}
	for i, l := range note {
		out[m.h-1-len(note)+i] = fit(noticeStyle.Render(l), sidebarW)
	}
	out[m.h-1] = fit(dimStyle.Render("⏎ a e d x v s H alt+s q"), sidebarW)
	return out
}

// formBox draws the pop-up as a bordered block of lines.
func (m model) formBox() []string {
	f := m.form
	inner := max(10, min(40, m.w-6))
	title := "add host"
	if f.edit {
		title = "edit host"
	}
	rows := []string{titleStyle.Render(title), ""}
	const labelW = 7 // "alias" plus a gap
	for i, fd := range hostFields {
		label := fit(fd.label, labelW)
		text := f.vals[i]
		room := max(1, inner-labelW-1)
		if r := []rune(text); len(r) > room {
			text = "…" + string(r[len(r)-room+1:])
		}
		if i == f.focus {
			rows = append(rows, curStyle.Render(label)+fit(text+cursorStyle.Render(" "), inner-labelW))
		} else {
			rows = append(rows, fit(dimStyle.Render(label)+text, inner))
		}
	}
	rows = append(rows, "")
	if f.err != "" {
		rows = append(rows, noticeStyle.Render(f.err))
	}
	rows = append(rows, dimStyle.Render("tab next · enter save · esc"))
	return framed(rows, inner)
}

// framed wraps rows to inner cells and draws them in a rounded border.
func framed(rows []string, inner int) []string {
	var lines []string
	for _, r := range rows {
		for _, l := range strings.Split(ansi.Wrap(r, inner, ""), "\n") {
			lines = append(lines, fit(l, inner))
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
	return strings.Split(box, "\n")
}

// hostDelete is the open delete question: the host and what depends on it.
type hostDelete struct {
	alias string
	use   hostuse.Usage
	err   error // the lookup failed, at least in part
}

// box draws the delete pop-up, with a warning when something uses the host.
func (d hostDelete) box(w int) []string {
	rows := []string{titleStyle.Render("delete host"), "", curStyle.Render(d.alias), ""}
	if len(d.use.Workspaces) > 0 {
		rows = append(rows,
			noticeStyle.Render("Used by workspaces:"),
			"  "+strings.Join(d.use.Workspaces, ", "),
			"Their ssh panes will be removed.", "")
	}
	if n := len(d.use.Panes); n > 0 {
		rows = append(rows, noticeStyle.Render(fmt.Sprintf("%d open pane(s) will be closed.", n)), "")
	}
	if d.err != nil {
		rows = append(rows, noticeStyle.Render("Could not check everything:"), ansi.Strip(d.err.Error()), "")
	}
	rows = append(rows, dimStyle.Render("y delete · any other key cancels"))
	return framed(rows, max(10, min(44, w-6)))
}

// popup is the pop-up to draw over the screen, if one is open.
func (m model) popup() []string {
	switch {
	case m.prompt != nil && m.prompt.kind == promptDelete:
		return m.deleteBox()
	case m.prompt != nil:
		return m.promptBox()
	case m.form != nil:
		return m.formBox()
	case m.hostDel != nil:
		return m.hostDel.box(m.w)
	case m.hostPick != nil:
		return m.hostPick.box(m.w)
	case m.hostShut != nil:
		return m.hostShut.box(m.w)
	}
	return nil
}

// overlay draws box over lines, centred.
func overlay(lines, box []string, w int) []string {
	if len(box) == 0 {
		return lines
	}
	bw := ansi.StringWidth(box[0])
	x, y := max(0, (w-bw)/2), max(0, (len(lines)-len(box))/2)
	out := append([]string(nil), lines...)
	for i, b := range box {
		if y+i >= len(out) {
			break
		}
		base := out[y+i]
		out[y+i] = ansi.Truncate(base, x, "") + "\x1b[0m" + b + "\x1b[0m" + ansi.TruncateLeft(base, x+bw, "")
	}
	return out
}

// hostLine is the alias alone: the form shows the rest.
func hostLine(h sshconfig.Host) string { return fit(" "+h.Alias, sidebarW) }

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/hostuse"
	"github.com/disun001/vantage/internal/sshconfig"
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
	// Edit changes the options of an existing Host block.
	Edit(sshconfig.Host) error
	// Delete removes a Host block.
	Delete(alias string) error
}

// HostUsage finds what depends on a host before it is deleted and closes the
// live panes connected to it. hostuse.Finder implements it.
type HostUsage interface {
	// Find reports the workspaces and open panes that use the alias. Partial
	// results may come back with an error.
	Find(alias string) (hostuse.Usage, error)
	// Close kills the open panes of the usage.
	Close(hostuse.Usage) error
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
	// hostSavedMsg reports a finished add or edit.
	hostSavedMsg struct {
		alias   string
		deleted bool
		err     error
	}
)

// hostField is one line of the add / edit form.
type hostField struct {
	label string
	ptr   func(*sshconfig.Host) *string
}

// hostFields are the form lines in order. Editing skips the first: the alias
// names the block and never changes.
var hostFields = []hostField{
	{"host", func(h *sshconfig.Host) *string { return &h.Alias }},
	{"hostname", func(h *sshconfig.Host) *string { return &h.HostName }},
	{"user", func(h *sshconfig.Host) *string { return &h.User }},
	{"port", func(h *sshconfig.Host) *string { return &h.Port }},
	{"jump", func(h *sshconfig.Host) *string { return &h.ProxyJump }},
}

// hostForm is the pop-up for adding or editing a host: every field is on
// screen, Tab moves between them, Enter saves, Esc drops everything.
type hostForm struct {
	edit  bool
	vals  [5]string // indexed like hostFields
	focus int
	err   string // why the last Enter did not save
}

// first is the first line the user can reach.
func (f hostForm) first() int {
	if f.edit {
		return 1
	}
	return 0
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
		return 0, "host is required"
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
	case "H", "esc", "h", "left":
		return m.toggleHosts()
	case "tab", "alt+l":
		if m.term != nil {
			m.focus = focusTerm
		}
	}
	return m, nil
}

// startDelete looks up what depends on the host, then asks. Without a usage
// finder it asks straight away.
func (m model) startDelete(alias string) (tea.Model, tea.Cmd) {
	m.notice = ""
	u := m.cfg.HostUse
	if u == nil {
		m.hostDel = alias
		return m, nil
	}
	return m, func() tea.Msg {
		use, err := u.Find(alias)
		return hostUsageMsg{alias: alias, use: use, err: err}
	}
}

// onHostUsage asks the delete question, with a warning when something uses
// the host.
func (m model) onHostUsage(msg hostUsageMsg) model {
	var warn []string
	if n := len(msg.use.Workspaces); n > 0 {
		warn = append(warn, "used by workspaces "+strings.Join(msg.use.Workspaces, ", ")+".")
	}
	if n := len(msg.use.Panes); n > 0 {
		warn = append(warn, fmt.Sprintf("%d open pane(s) will be closed.", n))
	}
	if msg.err != nil {
		warn = append(warn, "could not check everything: "+msg.err.Error())
	}
	m.hostDel, m.notice = msg.alias, ansi.Strip(strings.Join(warn, " "))
	return m
}

// onHostDelKey answers the delete question: y deletes, anything else cancels.
func (m model) onHostDelKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	alias := m.hostDel
	m.hostDel, m.notice = "", ""
	if k.String() != "y" {
		return m, nil
	}
	s, u := m.cfg.SSH, m.cfg.HostUse
	return m, func() tea.Msg {
		if err := s.Delete(alias); err != nil {
			return hostSavedMsg{alias: alias, deleted: true, err: err}
		}
		var err error
		if u != nil {
			// Look again: panes may have come or gone since the question.
			use, _ := u.Find(alias)
			if cerr := u.Close(use); cerr != nil {
				err = fmt.Errorf("host removed, but closing its panes failed: %w", cerr)
			}
		}
		return hostSavedMsg{alias: alias, deleted: true, err: err}
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
	f := hostForm{edit: true, focus: 1}
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
	last := len(hostFields) - 1
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.form = nil
	case tea.KeyEnter:
		return m.submitForm()
	case tea.KeyTab, tea.KeyDown:
		f.focus = f.first() + (f.focus-f.first()+1)%(last+1-f.first())
	case tea.KeyShiftTab, tea.KeyUp:
		n := last + 1 - f.first()
		f.focus = f.first() + (f.focus-f.first()+n-1)%n
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
	s, h := m.cfg.SSH, f.host()
	return func() tea.Msg {
		var err error
		if f.edit {
			err = s.Edit(h)
		} else {
			err = s.Add(h)
		}
		return hostSavedMsg{alias: h.Alias, err: err}
	}
}

func (m model) onHostSaved(msg hostSavedMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		m.notice = ansi.Strip(fmt.Sprintf("%s: %v", msg.alias, msg.err))
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
	out[m.h-1] = fit(dimStyle.Render("a e d H alt+s q"), sidebarW)
	if m.hostDel != "" {
		out[m.h-1] = fit(noticeStyle.Render("delete "+m.hostDel+"? y/n"), sidebarW)
	}
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
	const labelW = 10 // "hostname" plus a gap
	for i, fd := range hostFields {
		label := fit(fd.label, labelW)
		text := f.vals[i]
		room := max(1, inner-labelW-1)
		if r := []rune(text); len(r) > room {
			text = "…" + string(r[len(r)-room+1:])
		}
		switch {
		case f.edit && i == 0:
			rows = append(rows, dimStyle.Render(fit(label+text, inner)))
		case i == f.focus:
			rows = append(rows, curStyle.Render(label)+fit(text+cursorStyle.Render(" "), inner-labelW))
		default:
			rows = append(rows, fit(dimStyle.Render(label)+text, inner))
		}
	}
	rows = append(rows, "")
	if f.err != "" {
		rows = append(rows, ansi.Wrap(noticeStyle.Render(f.err), inner, ""))
	}
	rows = append(rows, dimStyle.Render("tab next · enter save · esc"))
	var lines []string
	for _, r := range rows {
		for _, l := range strings.Split(r, "\n") {
			lines = append(lines, fit(l, inner))
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
	return strings.Split(box, "\n")
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

// hostLine is the alias followed by its address.
func hostLine(h sshconfig.Host) string {
	meta := h.HostName
	if h.User != "" && meta != "" {
		meta = h.User + "@" + meta
	}
	return fit(" "+h.Alias+" "+dimStyle.Render(meta), sidebarW)
}

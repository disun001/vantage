package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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
	// hostSavedMsg reports a finished add or edit.
	hostSavedMsg struct {
		alias string
		err   error
	}
)

// hostField is one step of the add / edit form.
type hostField struct {
	label string
	ptr   func(*sshconfig.Host) *string
}

// hostFields are the form steps in order. Editing skips the first: the alias
// names the block and never changes.
var hostFields = []hostField{
	{"host: ", func(h *sshconfig.Host) *string { return &h.Alias }},
	{"hostname: ", func(h *sshconfig.Host) *string { return &h.HostName }},
	{"user: ", func(h *sshconfig.Host) *string { return &h.User }},
	{"port: ", func(h *sshconfig.Host) *string { return &h.Port }},
	{"jump: ", func(h *sshconfig.Host) *string { return &h.ProxyJump }},
}

// hostForm asks for a host's fields one line at a time at the bottom of the
// sidebar. Enter keeps the line and moves on, an empty line leaves the option
// unset, Esc drops everything.
type hostForm struct {
	edit bool
	host sshconfig.Host // answers so far; for an edit, the block's own values
	step int            // index into hostFields
	text string
}

func (f hostForm) line() string {
	label := hostFields[f.step].label
	room := max(1, sidebarW-len(label)-1)
	text := f.text
	if r := []rune(text); len(r) > room {
		text = "…" + string(r[len(r)-room+1:])
	}
	return fit(label+text+cursorStyle.Render(" "), sidebarW)
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
	case "H", "esc", "h", "left":
		return m.toggleHosts()
	case "tab", "alt+l":
		if m.term != nil {
			m.focus = focusTerm
		}
	}
	return m, nil
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
	m.form, m.notice = &hostForm{edit: true, host: own, step: 1, text: own.HostName}, ""
	return m
}

// onFormKey feeds a key to the open form.
func (m model) onFormKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := *m.form
	m.form = &f
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.form = nil
	case tea.KeyEnter:
		return m.advanceForm()
	case tea.KeyBackspace:
		if r := []rune(f.text); len(r) > 0 {
			m.form.text = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		m.form.text = ""
	case tea.KeyRunes:
		m.form.text += strings.Join(strings.Fields(string(k.Runes)), "") // no blanks in any field
	}
	return m, nil
}

// advanceForm stores the line, checks it, and moves to the next field or
// saves after the last one.
func (m model) advanceForm() (tea.Model, tea.Cmd) {
	f := *m.form
	f.text = strings.TrimSpace(f.text)
	if !f.edit && f.step == 0 && f.text == "" {
		m.form = nil // nothing typed for the alias: treat Enter as cancel
		return m, nil
	}
	next := f.host
	*hostFields[f.step].ptr(&next) = f.text
	if err := sshconfig.Validate(next); err != nil {
		m.notice = ansi.Strip(err.Error())
		return m, nil
	}
	m.notice = ""
	f.host = next
	if f.step == len(hostFields)-1 {
		m.form = nil
		return m, m.saveHost(f)
	}
	f.step++
	f.text = *hostFields[f.step].ptr(&f.host)
	m.form = &f
	return m, nil
}

// saveHost writes the finished form in the background.
func (m model) saveHost(f hostForm) tea.Cmd {
	s := m.cfg.SSH
	return func() tea.Msg {
		var err error
		if f.edit {
			err = s.Edit(f.host)
		} else {
			err = s.Add(f.host)
		}
		return hostSavedMsg{alias: f.host.Alias, err: err}
	}
}

func (m model) onHostSaved(msg hostSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = ansi.Strip(fmt.Sprintf("%s: %v", msg.alias, msg.err))
	} else {
		m.notice, m.hostCur = "", msg.alias
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
	out[m.h-1] = fit(dimStyle.Render("a e H alt+s q"), sidebarW)
	if m.form != nil {
		out[m.h-1] = m.form.line()
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

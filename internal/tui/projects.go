package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// ProjectStore is what the projects view needs. It is plain data both ways,
// so this package knows nothing of git or project files. Calls may be slow
// (git runs behind them) and are made in the background.
type ProjectStore interface {
	// List returns the project names, sorted.
	List() ([]string, error)
	// Detail reads the repo status of one project.
	Detail(name string) (ProjectInfo, error)
	// Create writes a new project and clones its repos.
	Create(ProjectDef) error
	// Remove deletes a project with its clones. Without force it refuses when
	// work would be lost.
	Remove(name string, force bool) error
}

// ProjectInfo is the details page of one project.
type ProjectInfo struct {
	Name  string
	Flat  bool
	Repos []RepoInfo
}

// RepoInfo is the state of one main clone, as text ready to show.
type RepoInfo struct {
	Name, Branch, Sync, Warn string
	Cloned, Dirty            bool
	Err                      string // why the status could not be read
}

// ProjectDef is a new project from the add form.
type ProjectDef struct {
	Name  string
	Flat  bool
	Repos []RepoDef
}

// RepoDef is one repo of a new project. Dir and Copy may be empty.
type RepoDef struct {
	URL, Dir string
	Copy     []string
}

type (
	projectsMsg struct { // the project list
		names []string
		err   error
	}
	projDetailMsg struct { // the details of project name
		name string
		info ProjectInfo
		err  error
	}
	projCreatedMsg struct { // a project add finished
		name string
		err  error
	}
	projRemovedMsg struct { // a project delete finished
		project string
		err     error
	}
)

func (m model) quit() (tea.Model, tea.Cmd) {
	if m.term != nil {
		m.term.Close()
	}
	return m, tea.Quit
}

func (m model) projPos() int {
	for i, n := range m.projects {
		if n == m.projCur {
			return i
		}
	}
	return 0
}

// moveProj moves the cursor and loads the details of the project it lands on.
func (m model) moveProj(delta int) (model, tea.Cmd) {
	if len(m.projects) == 0 {
		return m, nil
	}
	prev := m.projCur
	m.projCur = m.projects[max(0, min(m.projPos()+delta, len(m.projects)-1))]
	if m.projCur == prev {
		return m, nil
	}
	m.projInfo = nil
	return m, m.loadDetail(m.projCur)
}

func (m model) projScrolled() model {
	c, p := m.listCap(), m.projPos()
	if p < m.projTop {
		m.projTop = p
	}
	if p >= m.projTop+c {
		m.projTop = p - c + 1
	}
	m.projTop = max(0, min(m.projTop, len(m.projects)-c))
	return m
}

func (m model) loadProjects() tea.Cmd {
	s := m.cfg.Projects
	return func() tea.Msg {
		names, err := s.List()
		return projectsMsg{names: names, err: err}
	}
}

func (m model) loadDetail(name string) tea.Cmd {
	s := m.cfg.Projects
	return func() tea.Msg {
		info, err := s.Detail(name)
		return projDetailMsg{name: name, info: info, err: err}
	}
}

func (m model) onProjects(msg projectsMsg) (model, tea.Cmd) {
	if msg.err != nil {
		m.notice = ansi.Strip(msg.err.Error())
		return m, nil
	}
	m.projects = msg.names
	if !m.hasProject(m.projCur) {
		m.projCur, m.projInfo = "", nil
		if len(m.projects) > 0 {
			m.projCur = m.projects[0]
		}
	}
	if m.projCur == "" {
		return m, nil
	}
	return m, m.loadDetail(m.projCur)
}

func (m model) hasProject(name string) bool {
	for _, n := range m.projects {
		if n == name {
			return true
		}
	}
	return false
}

func (m model) onProjDetail(msg projDetailMsg) model {
	if msg.name != m.projCur {
		return m // the cursor moved on; this answer is stale
	}
	if msg.err != nil {
		m.notice, m.projInfo = ansi.Strip(msg.name+": "+msg.err.Error()), nil
		return m
	}
	m.projInfo = &msg.info
	return m
}

// toggleProjects switches the sidebar between workspaces and projects.
func (m model) toggleProjects() (model, tea.Cmd) {
	if m.cfg.Projects == nil {
		return m, nil
	}
	m.notice = ""
	if m.view == viewProjects {
		m.view = viewSessions
		return m, nil
	}
	m.view, m.focus = viewProjects, focusSidebar
	return m, m.loadProjects()
}

// onProjectKey handles a key in the projects view.
func (m model) onProjectKey(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "ctrl+c", "q":
		return m.quit()
	case "?":
		m.help = true
		return m, nil
	case "r":
		return m, m.loadProjects()
	case "H":
		return m.toggleHosts()
	case "j", "down":
		return m.moveProj(1)
	case "k", "up":
		return m.moveProj(-1)
	case "g", "home":
		return m.moveProj(-len(m.projects))
	case "G", "end":
		return m.moveProj(len(m.projects))
	case "a":
		m.pform, m.notice = newProjForm(), ""
	case "d":
		return m.startProjDel(), nil
	case "P", "esc", "h", "left":
		return m.toggleProjects()
	}
	return m, nil
}

func (m model) onProjCreated(msg projCreatedMsg) (tea.Model, tea.Cmd) {
	m.notice, m.okNotice = "", ""
	if msg.err != nil {
		m.notice = ansi.Strip(fmt.Sprintf("%s: %v", msg.name, msg.err))
	}
	m.projCur, m.projInfo = msg.name, nil
	return m, m.loadProjects()
}

// updateProjects handles the projects view's messages.
func (m model) updateProjects(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case projectsMsg:
		next, cmd := m.onProjects(msg)
		return next, cmd, true
	case projDetailMsg:
		return m.onProjDetail(msg), nil, true
	case projCreatedMsg:
		next, cmd := m.onProjCreated(msg)
		return next, cmd, true
	case projRemovedMsg:
		next, cmd := m.onProjRemoved(msg)
		return next, cmd, true
	}
	return m, nil, false
}

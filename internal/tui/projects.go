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
	// Detail reads the repo status and the tasks of one project.
	Detail(name string) (ProjectInfo, error)
	// Create writes a new project and clones its repos.
	Create(ProjectDef) error
	// NewTask creates a task's worktrees in the picked repos.
	NewTask(project, task string, repos []TaskPick) error
	// Remove deletes a project with its tasks and clones. Without force it
	// refuses when work would be lost.
	Remove(name string, force bool) error
	// RemoveTask deletes a task's worktrees, with the same refusal.
	RemoveTask(project, task string, force bool) error
	// Workspace returns the name of the task's workspace, creating the
	// workspace when none is linked to the task.
	Workspace(project, task string) (string, error)
}

// ProjectInfo is the details page of one project.
type ProjectInfo struct {
	Name  string
	Flat  bool
	Repos []RepoInfo
	Tasks []TaskInfo
}

// RepoInfo is the state of one main clone, as text ready to show.
type RepoInfo struct {
	Name, Branch, Sync, Warn string
	Cloned, Dirty            bool
	Err                      string // why the status could not be read
}

// TaskInfo is one task and the repos that have a worktree in it.
type TaskInfo struct {
	Name  string
	Repos []string
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

// TaskPick is one repo of a new task and its base branch ("" = default).
type TaskPick struct{ Repo, Base string }

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
	projRemovedMsg struct { // a project or task delete finished
		project, task string // task is empty for a project
		err           error
	}
	taskOpenMsg struct { // a task was created or picked: its workspace is ready
		workspace string
		err       error
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
	m.projInfo, m.taskCur = nil, 0
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
	m.taskCur = max(0, min(m.taskCur, len(msg.info.Tasks)-1))
	if len(msg.info.Tasks) == 0 {
		m.projFocus = false // the last task was deleted
	}
	return m
}

// toggleProjects switches the sidebar between workspaces and projects.
func (m model) toggleProjects() (model, tea.Cmd) {
	if m.cfg.Projects == nil {
		return m, nil
	}
	m.notice, m.projFocus = "", false
	if m.view == viewProjects {
		m.view = viewSessions
		return m, nil
	}
	m.view, m.focus = viewProjects, focusSidebar
	return m, m.loadProjects()
}

// onProjectKey handles a key in the projects view. The keys go to the
// project list, or to the task list once focus has stepped into the details.
func (m model) onProjectKey(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "ctrl+c", "q":
		return m.quit()
	case "?":
		m.help = true
		return m, nil
	case "n":
		return m.startTaskForm(), nil
	case "r":
		return m, m.loadProjects()
	case "H":
		m.projFocus = false
		return m.toggleHosts()
	}
	if m.projFocus {
		return m.onTaskKey(s)
	}
	switch s {
	case "j", "down":
		return m.moveProj(1)
	case "k", "up":
		return m.moveProj(-1)
	case "g", "home":
		return m.moveProj(-len(m.projects))
	case "G", "end":
		return m.moveProj(len(m.projects))
	case "enter", "l", "right", "tab":
		if m.projInfo != nil && len(m.projInfo.Tasks) > 0 {
			m.projFocus = true
		}
	case "a":
		m.pform, m.notice = newProjForm(), ""
	case "d":
		return m.startProjDel(""), nil
	case "P", "esc", "h", "left":
		return m.toggleProjects()
	}
	return m, nil
}

// onTaskKey handles a key while the task list has focus.
func (m model) onTaskKey(s string) (tea.Model, tea.Cmd) {
	n := len(m.projInfo.Tasks)
	switch s {
	case "j", "down":
		m.taskCur = min(m.taskCur+1, n-1)
	case "k", "up":
		m.taskCur = max(m.taskCur-1, 0)
	case "g", "home":
		m.taskCur = 0
	case "G", "end":
		m.taskCur = n - 1
	case "d":
		return m.startProjDel(m.projInfo.Tasks[m.taskCur].Name), nil
	case "enter", "l", "right":
		return m.openTask(m.projInfo.Tasks[m.taskCur].Name)
	case "esc", "h", "left", "tab":
		m.projFocus = false
	}
	return m, nil
}

// openTask opens the workspace of an existing task: it finds or creates it,
// starts its session if it is not live, and shows it.
func (m model) openTask(task string) (tea.Model, tea.Cmd) {
	m.notice = ""
	return m, m.taskCmd(m.projCur, task, nil)
}

// taskCmd creates the task first when picks is set, then makes sure its
// workspace exists and is running.
func (m model) taskCmd(project, task string, picks []TaskPick) tea.Cmd {
	s, restore := m.cfg.Projects, m.cfg.Restore
	return func() tea.Msg {
		if picks != nil {
			if err := s.NewTask(project, task, picks); err != nil {
				return taskOpenMsg{err: err}
			}
		}
		name, err := s.Workspace(project, task)
		if err != nil {
			return taskOpenMsg{err: err}
		}
		if restore != nil {
			if err := restore(name); err != nil {
				return taskOpenMsg{workspace: name, err: err}
			}
		}
		return taskOpenMsg{workspace: name}
	}
}

// onTaskOpen goes back to the workspaces sidebar on the task's workspace and
// shows it. On failure it stays on the details page and says why.
func (m model) onTaskOpen(msg taskOpenMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = ansi.Strip(msg.err.Error())
		if m.projCur != "" {
			return m, m.loadDetail(m.projCur) // a task may have been made before the failure
		}
		return m, nil
	}
	m.view, m.projFocus, m.notice = viewSessions, false, ""
	m.cur = key{msg.workspace, -1}
	return m.onReady(readyMsg{name: msg.workspace})
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
	case taskOpenMsg:
		next, cmd := m.onTaskOpen(msg)
		return next, cmd, true
	case projRemovedMsg:
		next, cmd := m.onProjRemoved(msg)
		return next, cmd, true
	}
	return m, nil, false
}

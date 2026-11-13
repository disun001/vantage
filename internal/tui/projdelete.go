package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// projDelete is the open delete question for a project.
type projDelete struct{ project string }

// startProjDel opens the delete question for the project under the cursor.
func (m model) startProjDel() model {
	if m.projCur == "" {
		return m
	}
	m.projDel, m.notice = &projDelete{project: m.projCur}, ""
	return m
}

// box draws the pop-up: what goes, and how to force past a refusal.
func (d projDelete) box(w int) []string {
	rows := []string{dangerTitle("delete project"), "", curStyle.Render(d.project), ""}
	rows = append(rows,
		"Removes the cloned repos and the",
		"project file. Workspace files stay.",
		"Refuses if work would be lost.", "",
		dimStyle.Render("y delete · f force · any other key cancels"))
	return framed(rows, max(10, min(44, w-6)))
}

// onProjDelKey answers the question: y deletes safely, f forces, anything
// else cancels.
func (m model) onProjDelKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	d := *m.projDel
	m.projDel, m.notice = nil, ""
	s := k.String()
	if s != "y" && s != "f" {
		return m, nil
	}
	force, store := s == "f", m.cfg.Projects
	return m, func() tea.Msg {
		return projRemovedMsg{project: d.project, err: store.Remove(d.project, force)}
	}
}

// onProjRemoved reloads after a delete. A refusal is shown on one line.
func (m model) onProjRemoved(msg projRemovedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = ansi.Strip(strings.Join(strings.Fields(msg.err.Error()), " "))
	}
	return m, m.loadProjects()
}

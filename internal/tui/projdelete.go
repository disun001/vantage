package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// projDelete is the open delete question for a project, or for one of its
// tasks when task is set.
type projDelete struct {
	project, task string
	tasks         int // tasks the project delete takes with it
}

// startProjDel opens the delete question for the project under the cursor, or
// for the task when one is given.
func (m model) startProjDel(task string) model {
	if m.projCur == "" {
		return m
	}
	d := &projDelete{project: m.projCur, task: task}
	if m.projInfo != nil {
		d.tasks = len(m.projInfo.Tasks)
	}
	m.projDel, m.notice = d, ""
	return m
}

// box draws the pop-up: what goes, and how to force past a refusal.
func (d projDelete) box(w int) []string {
	if d.task != "" {
		return framed([]string{
			dangerTitle("delete task"), "", curStyle.Render(d.project + " / " + d.task), "",
			"Removes its worktrees and their branches.",
			"Refuses if work would be lost.", "",
			dimStyle.Render("y delete · f force · any other key cancels"),
		}, max(10, min(44, w-6)))
	}
	rows := []string{dangerTitle("delete project"), "", curStyle.Render(d.project), ""}
	if d.tasks > 0 {
		rows = append(rows, noticeStyle.Render(fmt.Sprintf("%d task(s) and their worktrees go too.", d.tasks)))
	}
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
		var err error
		if d.task != "" {
			err = store.RemoveTask(d.project, d.task, force)
		} else {
			err = store.Remove(d.project, force)
		}
		return projRemovedMsg{project: d.project, task: d.task, err: err}
	}
}

// onProjRemoved reloads after a delete. A refusal is shown on one line.
func (m model) onProjRemoved(msg projRemovedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = ansi.Strip(strings.Join(strings.Fields(msg.err.Error()), " "))
	}
	if msg.task != "" {
		return m, m.loadDetail(msg.project)
	}
	return m, m.loadProjects()
}

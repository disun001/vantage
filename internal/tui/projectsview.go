package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// projectsSidebar draws the projects view's list.
func (m model) projectsSidebar() []string {
	out := make([]string, m.h)
	for i := range out {
		out[i] = strings.Repeat(" ", sidebarW)
	}
	out[0] = fit(m.pill(), sidebarW)
	out[1] = fit(hintStyle("projects"), sidebarW)
	note := m.noticeLines()
	cur := m.projPos()
	y := firstRowY
	for i := m.projTop; i < len(m.projects); i++ {
		if y >= m.h-1-len(note) {
			break
		}
		l := lineBuf{bg: m.rowBg(i == cur)}
		l.add(mocText, "", i == cur, " "+ansi.Truncate(m.projects[i], sidebarW-1, "…"))
		l.padTo(sidebarW)
		out[y] = l.String()
		y++
	}
	if len(m.projects) == 0 {
		out[firstRowY] = fit(hintStyle("no projects: a adds one"), sidebarW)
	}
	for i, l := range note {
		style := noticeStyle
		if m.notice == m.okNotice {
			style = dimStyle
		}
		out[m.h-1-len(note)+i] = fit(style.Render(l), sidebarW)
	}
	out[m.h-1] = fit(hintStyle("? keys"), sidebarW)
	return out
}

// projectDetails draws the right pane of the projects view: the repos of the
// project under the cursor with their state, then its tasks.
func (m model) projectDetails() []string {
	w, h := m.termW(), m.innerH()
	out := make([]string, h)
	for i := range out {
		out[i] = strings.Repeat(" ", w)
	}
	info := m.projInfo
	if info == nil || info.Name != m.projCur {
		msg := "loading…"
		if m.projCur == "" {
			msg = "no projects: a adds one"
		}
		out[h/2] = fit(strings.Repeat(" ", max(0, (w-len(msg))/2))+dimStyle.Render(msg), w)
		return out
	}
	head := []string{" " + curStyle.Render(info.Name) + flatTag(info), "", " " + dimStyle.Render("repos")}
	for _, r := range info.Repos {
		head = append(head, " "+repoLine(r, w-2))
	}
	head = append(head, "", " "+dimStyle.Render("tasks"))
	lines := append(head, m.taskLines(info, w, h-len(head)-1)...)
	for i := 0; i < len(lines) && i < h-1; i++ {
		out[i] = fit(lines[i], w)
	}
	hint := "n new task · a add project"
	if len(info.Tasks) > 0 {
		hint = "⏎ open task · " + hint
	}
	out[h-1] = fit(" "+dimStyle.Render(hint), w)
	return out
}

func flatTag(info *ProjectInfo) string {
	if info.Flat {
		return dimStyle.Render("  flat")
	}
	return ""
}

// repoLine is one repo: name, branch, sync state, then what needs a look.
func repoLine(r RepoInfo, w int) string {
	const nameW, branchW, syncW = 14, 14, 12
	name := fit(ansi.Truncate(r.Name, nameW-1, "…"), nameW)
	switch {
	case r.Err != "":
		return fit(name+noticeStyle.Render(r.Err), w)
	case !r.Cloned:
		return fit(name+dimStyle.Render("not cloned (vantage project sync)"), w)
	}
	extra := ""
	if r.Dirty {
		extra = paint(mocYellow, "").Render("dirty ")
	}
	if r.Warn != "" {
		extra += noticeStyle.Render(r.Warn)
	}
	return fit(name+fit(ansi.Truncate(r.Branch, branchW-1, "…"), branchW)+
		dimStyle.Render(fit(r.Sync, syncW))+extra, w)
}

// taskLines lists the tasks, scrolled so the cursor stays inside room lines.
func (m model) taskLines(info *ProjectInfo, w, room int) []string {
	if len(info.Tasks) == 0 {
		return []string{" " + dimStyle.Render("none yet: n creates one")}
	}
	room = max(1, room)
	start := max(0, min(m.taskCur-room+1, len(info.Tasks)-room))
	var out []string
	for i := start; i < len(info.Tasks) && len(out) < room; i++ {
		t := info.Tasks[i]
		text := fmt.Sprintf(" %-22s %s", ansi.Truncate(t.Name, 22, "…"), strings.Join(t.Repos, ", "))
		if m.projFocus && i == m.taskCur {
			out = append(out, selStyle.Render(fit(text, w)))
			continue
		}
		out = append(out, fit(text, w))
	}
	return out
}

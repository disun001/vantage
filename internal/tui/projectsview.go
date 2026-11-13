package tui

import (
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
// project under the cursor with their state.
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
	for i := 0; i < len(head) && i < h-1; i++ {
		out[i] = fit(head[i], w)
	}
	out[h-1] = fit(" "+dimStyle.Render("a add project · d delete · r refresh"), w)
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

package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Text styles shared by the sidebar and the pop-ups (see popupstyle.go).
var (
	dimStyle    = paint(mocOverlay, "")
	curStyle    = paint(mocLavender, "").Bold(true) // the focused field's label, a name asked about
	cursorStyle = paint(mocBase, mocText)           // the text cursor, a block
	selStyle    = paint(mocText, mocSurf1).Bold(true)
	noticeStyle = paint(mocRed, "")
)

func (m model) View() string {
	if m.w < sidebarW+3 || m.h < 3 {
		return "window too small"
	}
	side := m.sidebar()
	right := m.right()
	edge := borderIdle
	if m.focus == focusTerm {
		edge = borderActive
	}
	frame := paint(edge, "")
	bar := frame.Render
	lines := make([]string, m.h)
	rule := strings.Repeat("─", m.termW())
	for i := range lines {
		switch i {
		case 0:
			lines[i] = side[i] + bar("╭"+rule+"╮")
		case m.h - 1:
			lines[i] = side[i] + bar("╰"+rule+"╯")
		default:
			lines[i] = side[i] + bar("│") + right[i-1] + bar("│")
		}
	}
	if box := m.popup(); box != nil {
		lines = overlay(lines, box, m.w)
	}
	return strings.Join(lines, "\n")
}

// fit truncates or pads s to exactly w cells.
func fit(s string, w int) string {
	if n := ansi.StringWidth(s); n > w {
		return ansi.Truncate(s, w, "…")
	} else if n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func (m model) right() []string {
	if m.view == viewProjects {
		return m.projectDetails()
	}
	w := m.termW()
	out := make([]string, m.innerH())
	if m.term == nil {
		msg := "enter: open a session"
		for i := range out {
			out[i] = strings.Repeat(" ", w)
		}
		out[len(out)/2] = fit(strings.Repeat(" ", max(0, (w-len(msg))/2))+dimStyle.Render(msg), w)
		return out
	}
	lines := m.term.Lines(m.focus == focusTerm)
	if n := m.bar.lines; n > 0 && n < len(lines) {
		if m.bar.top {
			lines = lines[n:]
		} else {
			lines = lines[:len(lines)-n]
		}
	}
	for i := range out {
		if i < len(lines) {
			out[i] = fit(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}

func (m model) sidebar() []string {
	if m.view == viewHosts {
		return m.hostsSidebar()
	}
	if m.view == viewProjects {
		return m.projectsSidebar()
	}
	out := make([]string, m.h)
	for i := range out {
		out[i] = strings.Repeat(" ", sidebarW)
	}
	out[0] = fit(m.pill(), sidebarW)
	out[1] = m.tabLine()

	note := m.noticeLines()
	items := m.items()
	cur := m.pos(items)
	for y, i := range span(m.flatItems(items), m.top, m.listCap()) {
		if i < 0 {
			out[firstRowY+y] = ruleLine()
			continue
		}
		out[firstRowY+y] = m.itemLine(items[i], i == cur)
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

// maxNoticeLines caps how much of the sidebar an error may take.
const maxNoticeLines = 5

// noticeLines wraps the notice to the sidebar width so a long error (a
// failed restore names a file and a reason) is readable in full instead of
// cut off. Past the cap the last line ends with an ellipsis.
func (m model) noticeLines() []string {
	if m.notice == "" || m.h < 8 {
		return nil
	}
	lines := strings.Split(ansi.Wrap(m.notice, sidebarW, ""), "\n")
	if len(lines) > maxNoticeLines {
		lines = lines[:maxNoticeLines]
		lines[maxNoticeLines-1] = ansi.Truncate(lines[maxNoticeLines-1], sidebarW-1, "") + "…"
	}
	return lines
}

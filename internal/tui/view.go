package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	dimStyle    = lipgloss.NewStyle().Faint(true)
	liveDot     = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("●")
	savedDot    = lipgloss.NewStyle().Faint(true).Render("○")
	titleStyle  = lipgloss.NewStyle().Bold(true)
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	curStyle    = lipgloss.NewStyle().Bold(true)
	noticeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func (m model) View() string {
	if m.w < sidebarW+3 || m.h < 3 {
		return "window too small"
	}
	side := m.sidebar()
	right := m.right()
	div := dimStyle.Render("│")
	var b strings.Builder
	for i := 0; i < m.h; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(side[i])
		b.WriteString(div)
		b.WriteString(right[i])
	}
	return b.String()
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
	w := m.termW()
	out := make([]string, m.h)
	if m.term == nil {
		msg := "enter: open a session"
		for i := range out {
			out[i] = strings.Repeat(" ", w)
		}
		out[m.h/2] = fit(strings.Repeat(" ", max(0, (w-len(msg))/2))+dimStyle.Render(msg), w)
		return out
	}
	lines := m.term.Lines(m.focus == focusTerm)
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
	out := make([]string, m.h)
	for i := range out {
		out[i] = strings.Repeat(" ", sidebarW)
	}
	title := "vantage"
	if m.focus == focusSidebar {
		title = "▌vantage"
	}
	out[0] = fit(titleStyle.Render(title), sidebarW)

	y := 2
	for i, r := range m.rows {
		if y >= m.h-2 {
			break
		}
		out[y] = m.rowLine(i, r)
		y++
	}
	if m.notice != "" && m.h > 4 {
		out[m.h-2] = fit(noticeStyle.Render(m.notice), sidebarW)
	}
	out[m.h-1] = fit(dimStyle.Render("j/k enter  alt+s focus  q"), sidebarW)
	return out
}

func (m model) rowLine(i int, r row) string {
	dot := savedDot
	meta := ""
	if r.Live {
		dot = liveDot
		meta = fmt.Sprintf("%dw %dp", r.Windows, r.Panes)
	}
	nameW := sidebarW - 4 - len(meta) - 1
	name := fit(ansi.Truncate(r.Name, nameW, "…"), max(nameW, 1))
	if r.Name == m.current {
		name = curStyle.Render(name)
	}
	line := " " + dot + " " + name + " " + dimStyle.Render(meta)
	line = fit(line, sidebarW)
	if i == m.cursor {
		if m.focus == focusSidebar {
			return cursorStyle.Render(ansi.Strip(line))
		}
		return lipgloss.NewStyle().Underline(true).Render(ansi.Strip(line))
	}
	return line
}

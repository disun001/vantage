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
	out := make([]string, m.h)
	for i := range out {
		out[i] = strings.Repeat(" ", sidebarW)
	}
	title := "vantage"
	if m.focus == focusSidebar {
		title = "▌vantage"
	}
	out[0] = fit(titleStyle.Render(title), sidebarW)

	note := m.noticeLines()
	items := m.items()
	cur := m.pos(items)
	y := firstRowY
	for i := m.top; i < len(items); i++ {
		if y >= m.h-1-len(note) {
			break
		}
		out[y] = m.itemLine(items[i], i == cur)
		y++
	}
	for i, l := range note {
		out[m.h-1-len(note)+i] = fit(noticeStyle.Render(l), sidebarW)
	}
	out[m.h-1] = fit(dimStyle.Render("j/k enter  space  alt+s  q"), sidebarW)
	return out
}

func (m model) itemLine(it item, selected bool) string {
	r := m.rows[it.Row]
	if it.Win >= 0 {
		return m.highlight(m.winLine(r.Wins[it.Win]), selected)
	}
	return m.highlight(m.rowLine(r), selected)
}

// rowLine draws a session: fold arrow (live sessions with windows), dot,
// name, and the window and pane counts.
func (m model) rowLine(r row) string {
	dot := savedDot
	meta, arrow := "", " "
	if r.Live {
		dot = liveDot
		meta = fmt.Sprintf("%dw %dp", r.Windows, r.Panes)
		if len(r.Wins) > 0 {
			arrow = dimStyle.Render("▸")
			if m.expanded(r) {
				arrow = dimStyle.Render("▾")
			}
		}
	}
	nameW := sidebarW - 4 - len(meta) - 1
	name := fit(ansi.Truncate(r.Name, nameW, "…"), max(nameW, 1))
	if r.Name == m.current {
		name = curStyle.Render(name)
	}
	return fit(arrow+" "+dot+" "+name+" "+dimStyle.Render(meta), sidebarW)
}

// winLine draws a window under its session. The session's current window is
// bold and marked with * like tmux's own status bar.
func (m model) winLine(w win) string {
	meta := fmt.Sprintf("%dp", w.Panes)
	label := fmt.Sprintf("%d:%s", w.Index, w.Name)
	if w.Active {
		label += "*"
	}
	labelW := max(1, sidebarW-4-len(meta)-1)
	label = fit(ansi.Truncate(label, labelW, "…"), labelW)
	if w.Active {
		label = curStyle.Render(label)
	}
	return fit("    "+label+" "+dimStyle.Render(meta), sidebarW) // label lines up with the session name
}

// highlight marks the cursor line: reversed with focus, underlined without.
func (m model) highlight(line string, selected bool) string {
	if !selected {
		return line
	}
	if m.focus == focusSidebar {
		return cursorStyle.Render(ansi.Strip(line))
	}
	return lipgloss.NewStyle().Underline(true).Render(ansi.Strip(line))
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

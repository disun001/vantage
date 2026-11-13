package tui

import (
	"fmt"
	"strconv"
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
	if m.focus == focusTerm || (m.view == viewProjects && m.projFocus) {
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
	out[1] = fit(hintStyle("workspaces"), sidebarW)

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
		style := noticeStyle
		if m.notice == m.okNotice {
			style = dimStyle
		}
		out[m.h-1-len(note)+i] = fit(style.Render(l), sidebarW)
	}
	out[m.h-1] = fit(hintStyle("? keys"), sidebarW)
	return out
}

func (m model) itemLine(it item, selected bool) string {
	r := m.rows[it.Row]
	if it.Win >= 0 {
		return m.winLine(r, it.Win, selected)
	}
	return m.rowLine(r, selected)
}

// rowLine draws a session: fold arrow (live sessions with windows), dot,
// name, and the window and pane counts. The session on screen has its name
// in a mauve pill, the one mark that is not yellow, so it stands out and
// the yellow stays with the window.
func (m model) rowLine(r row, selected bool) string {
	l := lineBuf{bg: m.rowBg(selected)}
	meta, arrow, dot, dotFG := "", " ", "○", mocOverlay
	if r.Live {
		meta = fmt.Sprintf("%dw %dp", r.Windows, r.Panes)
		dot, dotFG = "●", mocGreen
		if len(r.Wins) > 0 {
			arrow = "▸"
			if m.expanded(r) {
				arrow = "▾"
			}
		}
	}
	// A collapsed session shows its chat state, since its windows are hidden.
	chat := ""
	if r.Live && !m.expanded(r) {
		chat = r.Chat
	}
	nameW := max(sidebarW-4-len(meta)-1-chatCells(chat), 1)
	l.add(mocOverlay, "", false, arrow+" ")
	l.add(dotFG, "", false, dot)
	if r.Name == m.current {
		// A plain gap after the dot keeps the pill from touching it.
		l.add(mocText, "", false, " ")
		l.add(mocBase, mocMauve, true, " "+ansi.Truncate(r.Name, nameW-2, "…")+" ")
	} else {
		fg := mocText
		if !r.Live {
			fg = mocSub
		}
		l.add(fg, "", false, "  "+ansi.Truncate(r.Name, nameW-1, "…"))
	}
	if meta != "" {
		l.padTo(sidebarW - len(meta) - 1 - chatCells(chat))
		l.addChat(chat, "")
		l.add(mocOverlay, "", false, meta)
	}
	l.padTo(sidebarW)
	return l.String()
}

// winLine draws window i of a session as one tile hanging off the session's
// dot: a tree guide, the number in a chip, then the title on a block that
// runs to the pane count, like the tmux status bar. Only the active window of
// the session on screen has the yellow chip; every other window looks the
// same, whichever session it is in.
func (m model) winLine(r row, i int, selected bool) string {
	w := r.Wins[i]
	l := lineBuf{bg: m.rowBg(selected)}
	meta := fmt.Sprintf("%dp", w.Panes)
	digits := len(strconv.Itoa(r.Wins[len(r.Wins)-1].Index))
	for _, o := range r.Wins {
		digits = max(digits, len(strconv.Itoa(o.Index)))
	}
	num := fmt.Sprintf(" %*d ", digits, w.Index)
	chip, bold := mocOverlay, false
	if w.Active && r.Name == m.current {
		chip, bold = mocYellow, true
	}
	tile := mocSurf0
	if selected {
		tile = mocSurf1
	}
	guide := "├"
	if i == len(r.Wins)-1 {
		guide = "└"
	}
	l.add(mocOverlay, "", false, "  "+guide) // hangs under the session's dot
	l.add(mocBase, chip, bold, num)
	nameW := max(1, sidebarW-1-l.w-3-len(meta)-chatCells(w.Chat))
	l.add(mocText, tile, bold, " "+ansi.Truncate(w.Name, nameW, "…"))
	l.padBg(sidebarW-len(meta)-2-chatCells(w.Chat), tile)
	l.addChat(w.Chat, tile)
	l.add(mocOverlay2, tile, false, meta+" ")
	l.padTo(sidebarW)
	return l.String()
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

// chatMark is the glyph and color of a chat status in the sidebar.
var chatMark = map[string]struct{ glyph, fg string }{
	"idle":        {"◇", mocOverlay2},
	"running":     {"◆", mocBlue},
	"error":       {"✖", mocRed},
	"interrupted": {"⊘", mocLavender},
}

// chatCells is how many columns a chat marker and its gap take.
func chatCells(status string) int {
	if _, ok := chatMark[status]; ok {
		return 2
	}
	return 0
}

// addChat appends the marker for a chat status, if it has one.
func (l *lineBuf) addChat(status, bg string) {
	if mk, ok := chatMark[status]; ok {
		l.add(mk.fg, bg, false, mk.glyph+" ")
	}
}

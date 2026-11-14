package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func repeat(s string, n int) string { return strings.Repeat(s, n) }

// entry is one top-level line of any list: a session, a host or a project.
// All three share the layout: a cursor mark, a fold arrow, the name, then on
// the right a plain dim note and a status dot.
type entry struct {
	name     string
	nameFG   string
	fold     string // "▸", "▾" or " "
	meta     string // right-hand note, plain text
	dot      string // status glyph, empty for none
	dotFG    string
	cursor   bool // the cursor is on this line
	onScreen bool // the session shown in the terminal: mauve, bold
}

// foldW is the cursor mark and fold arrow before a name: " ❯▾ ".
const foldW = 4

func (m model) entryLine(e entry) string {
	l := lineBuf{}
	l.add(mocText, "", false, " ")
	m.markCell(&l, e.cursor)
	l.add(mocOverlay, "", false, e.fold+" ")
	fg, bold := e.nameFG, e.cursor
	if e.onScreen {
		fg, bold = mocMauve, true
	}
	room := max(sidebarW-foldW-plainTailW(e.meta, e.dot)-1, 1)
	l.add(fg, "", bold, ansi.Truncate(e.name, room, "…"))
	l.plainTail(e.meta, e.dot, e.dotFG)
	return l.String()
}

// plainTailW is the width of what plainTail draws.
func plainTailW(meta, dot string) int {
	w := 1 // the margin
	if meta != "" {
		w += lineWidth(meta) + 1
	}
	if dot != "" {
		w += 2
	}
	return w
}

// plainTail draws the right edge: a dim note, the status dot, a margin.
func (l *lineBuf) plainTail(meta, dot, dotFG string) {
	l.padTo(sidebarW - plainTailW(meta, dot))
	if meta != "" {
		l.add(mocOverlay2, "", false, " "+meta)
	}
	if dot != "" {
		l.add(dotFG, "", false, " "+dot)
	}
	l.add(mocText, "", false, " ")
}

// markCell draws the cursor mark, or a plain gap where the cursor is not.
// The mark is mauve with the sidebar focused and dim without.
func (m model) markCell(l *lineBuf, on bool) {
	if !on {
		l.add(mocText, "", false, " ")
		return
	}
	fg := mocOverlay
	if m.focus == focusSidebar {
		fg = mocMauve
	}
	l.add(fg, "", true, "❯")
}

func (m model) itemLine(it item, selected bool) string {
	r := m.rows[it.Row]
	if it.Win >= 0 {
		return m.winLine(r, it.Win, selected)
	}
	return m.rowLine(r, selected)
}

// rowLine draws a session. The one on screen has its name in mauve bold; a
// live session has a green dot on the right, a saved one a dim ring.
func (m model) rowLine(r row, selected bool) string {
	e := entry{name: r.Name, nameFG: mocSub, fold: " ", dot: iconRing, dotFG: mocOverlay, cursor: selected, onScreen: r.Name == m.current}
	if r.Live {
		e.nameFG, e.meta, e.dot, e.dotFG = mocText, fmt.Sprintf("%dw %dp", r.Windows, r.Panes), iconDot, mocGreen
		if len(r.Wins) > 0 {
			e.fold = "▸"
			if m.expanded(r) {
				e.fold = "▾"
			}
		}
	}
	return m.entryLine(e)
}

// winLine draws window i of a session as a plain line hanging off it: a tree
// guide, the window number and title, and the pane count on the right. Only
// the active window of the session on screen is yellow.
func (m model) winLine(r row, i int, selected bool) string {
	w := r.Wins[i]
	l := lineBuf{}
	guide := "├"
	if i == len(r.Wins)-1 {
		guide = "└"
	}
	meta := fmt.Sprintf("%dp", w.Panes)
	digits := 1
	for _, o := range r.Wins {
		digits = max(digits, len(strconv.Itoa(o.Index)))
	}
	fg, bold := mocSub, selected
	if w.Active && r.Name == m.current {
		fg, bold = mocYellow, true
	}
	l.add(mocText, "", false, " ")
	m.markCell(&l, selected)
	l.add(mocText, "", false, " ")
	l.add(mocOverlay, "", false, guide+" ")
	room := max(sidebarW-foldW-2-len(meta)-3, 1)
	title := ansi.Truncate(fmt.Sprintf("%*d %s", digits, w.Index, w.Name), room, "…")
	l.add(fg, "", bold, title)
	l.padTo(sidebarW - len(meta) - 2)
	l.add(mocOverlay, "", false, " "+meta+" ")
	return l.String()
}

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// bind is one key and what it does, as shown in the help pop-up.
type bind struct{ key, does string }

// helpGroup is a titled run of binds, so a key is easy to find by task.
type helpGroup struct {
	title string
	binds []bind
}

const helpGap = 2 // cells between the two columns

var (
	moveGroup = helpGroup{"Move", []bind{
		{"j/k", "down / up"},
		{"g/G", "first / last"},
	}}
	focusGroup = helpGroup{"Focus", []bind{
		{"alt+s", "sidebar / tmux"},
		{"tab", "to tmux (alt+l)"},
		{"alt+h", "from tmux edge"},
		{"alt+/", "help, from tmux too"},
		{"alt+r", "rename pane"},
	}}
)

// helpGroups is what the pop-up lists for the view on screen. The sidebar
// footer shows only "? keys", so this is where each key is explained.
func (m model) helpGroups() []helpGroup {
	switch m.view {
	case viewProjects:
		return projectHelp()
	case viewHosts:
		return hostHelp()
	}
	return []helpGroup{
		moveGroup,
		{"Sessions", []bind{
			{"⏎", "show in tmux"},
			{"l", "open / step in"},
			{"h", "fold"},
			{"space", "fold / unfold"},
			{"r", "refresh"},
		}},
		{"Workspaces", []bind{
			{"n", "new"},
			{"R", "rename session / window"},
			{"K", "stop session, keep file"},
			{"d", "delete"},
			{"S", "save all live sessions"},
		}},
		focusGroup,
		{"App", []bind{
			{"H", "ssh hosts"},
			{"P", "projects"},
			{"?", "this help"},
			{"q", "quit"},
		}},
	}
}

// groupHeight is the lines the groups take, one blank line between them.
func groupHeight(gs []helpGroup) int {
	n := 0
	for _, g := range gs {
		n += 1 + len(g.binds)
	}
	return n + max(0, len(gs)-1)
}

// splitGroups cuts gs into two runs of nearly equal height, in order.
func splitGroups(gs []helpGroup) (left, right []helpGroup) {
	best, at := -1, len(gs)
	for i := 1; i < len(gs); i++ {
		if h := max(groupHeight(gs[:i]), groupHeight(gs[i:])); best < 0 || h < best {
			best, at = h, i
		}
	}
	return gs[:at], gs[at:]
}

// helpColumn draws groups as lines of one column: bold titles, keys padded to
// keyW. The lines are as wide as the widest, so columns line up.
func helpColumn(gs []helpGroup, keyW int) (lines []string, width int) {
	for i, g := range gs {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, curStyle.Render(g.title))
		width = max(width, len(g.title))
		for _, b := range g.binds {
			pad := strings.Repeat(" ", keyW-len([]rune(b.key)))
			lines = append(lines, paint(mocYellow, "").Render(b.key)+pad+" "+b.does)
			width = max(width, keyW+1+len([]rune(b.does)))
		}
	}
	return lines, width
}

// helpBox is the pop-up of keys. It uses two columns when the window is wide
// enough and one otherwise, and cuts the list short on a window too low.
func (m model) helpBox() []string {
	gs := m.helpGroups()
	keyW := 0
	for _, g := range gs {
		for _, b := range g.binds {
			keyW = max(keyW, len([]rune(b.key)))
		}
	}
	room := max(10, m.w-6)

	var body []string
	inner := 0
	l, r := splitGroups(gs)
	ll, lw := helpColumn(l, keyW)
	rl, rw := helpColumn(r, keyW)
	if len(r) > 0 && lw+helpGap+rw <= room {
		n := max(len(ll), len(rl))
		for i := 0; i < n; i++ {
			a, b := "", ""
			if i < len(ll) {
				a = ll[i]
			}
			if i < len(rl) {
				b = rl[i]
			}
			body = append(body, fit(a, lw)+strings.Repeat(" ", helpGap)+b)
		}
		inner = lw + helpGap + rw
	} else {
		body, inner = helpColumn(gs, keyW)
		inner = min(inner, room)
	}

	rows := append([]string{popTitle("keys"), ""}, body...)
	if limit := max(1, m.h-2-2); len(rows) > limit { // 2 for the border, 2 for the footer
		rows = append(rows[:limit-1], dimStyle.Render("…"))
	}
	rows = append(rows, "", dimStyle.Render("esc, ? or q closes"))
	return framed(rows, max(inner, len("esc, ? or q closes")))
}

// onHelpKey owns every key while the help pop-up is open.
func (m model) onHelpKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "?", "q":
		m.help = false
	}
	return m, nil
}

func projectHelp() []helpGroup {
	return []helpGroup{
		moveGroup,
		{"Projects", []bind{
			{"a", "add project"},
			{"d", "delete project"},
			{"h/esc", "back"},
			{"r", "refresh"},
		}},
		{"App", []bind{
			{"P", "back to workspaces"},
			{"H", "ssh hosts"},
			{"?", "this help"},
			{"q", "quit"},
		}},
	}
}

func hostHelp() []helpGroup {
	return []helpGroup{
		moveGroup,
		{"Connect", []bind{
			{"⏎", "go to pane / open ssh"},
			{"v", "ssh split beside"},
			{"s", "ssh split below"},
			{"x", "close host's panes"},
			{"u", "unlock password vault"},
		}},
		{"Hosts", []bind{
			{"a", "add"},
			{"e", "edit"},
			{"d", "delete"},
			{"r", "refresh"},
		}},
		{"App", []bind{
			{"H/esc", "back to workspaces"},
			{"?", "this help"},
			{"q", "quit"},
		}},
	}
}

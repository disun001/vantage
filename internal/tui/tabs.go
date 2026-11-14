package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Nerd Font glyphs for the sidebar. A terminal without a Nerd Font shows
// boxes for these.
const (
	iconDot  = "" // a live session or a connected host
	iconRing = "" // a saved or idle one
)

// tab is one of the three lists the sidebar can show.
type tab struct {
	view sidebarView
	key  string // the key that opens it, empty for the home list
	name string
}

var tabs = []tab{
	{viewSessions, "", "ws"},
	{viewHosts, "H", "hosts"},
	{viewProjects, "P", "proj"},
}

// tabLine draws the tab strip under the title: the open list on a lifted
// block, the others dim, each with the key that opens it.
func (m model) tabLine() string {
	l := lineBuf{}
	l.add(mocText, "", false, " ")
	for _, t := range tabs {
		label := " " + t.name + " "
		if t.key != "" {
			label = " " + t.key + " " + t.name + " "
		}
		if t.view == m.view {
			l.add(mocText, mocSurf1, true, label)
		} else {
			l.add(mocOverlay, "", false, label)
		}
	}
	l.padTo(sidebarW)
	return l.String()
}

func lineWidth(s string) int { return len([]rune(s)) }

// countLabel is "1 repo" or "n repos".
func countLabel(n int) string {
	if n == 1 {
		return "1 repo"
	}
	return fmt.Sprintf("%d repos", n)
}

// loadHostsIfShown refreshes the connection dots on the timer, only while the
// hosts list is on screen.
func (m model) loadHostsIfShown() tea.Cmd {
	if m.view != viewHosts || m.cfg.SSH == nil {
		return nil
	}
	if _, ok := m.cfg.HostUse.(hostLinker); !ok {
		return nil
	}
	return m.loadHosts()
}

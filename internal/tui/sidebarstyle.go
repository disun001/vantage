package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Catppuccin Mocha, the same colors as the tmux theme, so the sidebar reads as
// part of the status bar: a window number in a colored chip, then its title.
const (
	mocBase     = "#1e1e2e"
	mocMantle   = "#181825"
	mocSurf0    = "#313244"
	mocSurf1    = "#45475a"
	mocOverlay  = "#6c7086"
	mocOverlay2 = "#9399b2"
	mocSub      = "#a6adc8"
	mocText     = "#cdd6f4"
	mocYellow   = "#f9e2af"
	mocMauve    = "#cba6f7"
	mocGreen    = "#a6e3a1"
	mocBlue     = "#89b4fa"
	mocRed      = "#f38ba8"
	mocLavender = "#b4befe"
)

// paint is a foreground and background pair; an empty bg leaves the
// terminal's own.
func paint(fg, bg string) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(fg))
	if bg != "" {
		s = s.Background(lipgloss.Color(bg))
	}
	return s
}

// hintStyle draws the key hints at the foot of the sidebar.
func hintStyle(s string) string { return paint(mocOverlay, "").Render(s) }

// pill is the title tag, the mode of the moment (see modeTag).
func (m model) pill() string {
	label, bg := m.modeTag()
	return paint(mocBase, bg).Bold(true).Render(" " + label + " ")
}

// rowBg is the fill behind the cursor line: a clear one with the sidebar
// focused, a faint one without.
func (m model) rowBg(selected bool) string {
	switch {
	case !selected:
		return ""
	case m.focus == focusSidebar:
		return mocSurf0
	}
	return mocMantle
}

// lineBuf builds one sidebar line from styled pieces. It tracks the width so
// padding can carry the row's background to the edge.
type lineBuf struct {
	sb strings.Builder
	w  int
	bg string // row fill, used by pieces that set no background of their own
}

// add appends text in the given colors. An empty bg takes the row fill.
func (l *lineBuf) add(fg, bg string, bold bool, text string) {
	if text == "" {
		return
	}
	if bg == "" {
		bg = l.bg
	}
	l.sb.WriteString(paint(fg, bg).Bold(bold).Render(text))
	l.w += ansi.StringWidth(text)
}

// padTo fills with spaces up to width w.
func (l *lineBuf) padTo(w int) {
	n := w - l.w
	if n <= 0 {
		return
	}
	if l.bg == "" {
		l.sb.WriteString(strings.Repeat(" ", n))
		l.w += n
		return
	}
	l.add(mocText, "", false, strings.Repeat(" ", n))
}

// padBg fills with spaces up to width w on the given background.
func (l *lineBuf) padBg(w int, bg string) {
	if n := w - l.w; n > 0 {
		l.add(mocText, bg, false, strings.Repeat(" ", n))
	}
}

func (l *lineBuf) String() string { return l.sb.String() }

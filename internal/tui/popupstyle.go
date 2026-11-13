package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// popTitle is a pop-up's title tag, a mauve pill like the session pill.
func popTitle(text string) string {
	return paint(mocBase, mocMauve).Bold(true).Render(" " + text + " ")
}

// dangerTitle is the title of a pop-up that asks before something goes.
func dangerTitle(text string) string {
	return paint(mocBase, mocRed).Bold(true).Render(" " + text + " ")
}

// styleSeq is the escape sequence that starts s, or "" when colors are off.
func styleSeq(s lipgloss.Style) string {
	out := s.Render("x")
	return out[:strings.Index(out, "x")]
}

// framed wraps rows to inner cells and draws them in a rounded border on a
// mantle panel. Every piece of a row ends with a reset, so the panel colors
// are started again after each one.
func framed(rows []string, inner int) []string {
	const reset = "\x1b[0m"
	base := styleSeq(paint(mocText, mocMantle))
	edge := paint(mocOverlay, mocMantle)

	var lines []string
	for _, r := range rows {
		for _, l := range strings.Split(ansi.Wrap(r, inner, ""), "\n") {
			l = fit(l, inner)
			body := base + strings.ReplaceAll(" "+l+" ", reset, reset+base) + reset
			lines = append(lines, edge.Render("│")+body+edge.Render("│"))
		}
	}
	bar := strings.Repeat("─", inner+2)
	out := []string{edge.Render("╭" + bar + "╮")}
	out = append(out, lines...)
	return append(out, edge.Render("╰"+bar+"╯"))
}

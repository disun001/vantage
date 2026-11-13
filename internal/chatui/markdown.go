package chatui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// markdown renders assistant text as terminal markdown for a given width. A
// nil renderer (or a render failure) falls back to plain wrapped text, so a
// bad message never blanks the transcript.
type markdown struct {
	r *glamour.TermRenderer
}

// newMarkdown builds a renderer. dark picks the palette; it is decided once,
// before the Bubble Tea program owns the terminal, because asking the
// terminal for its background later would eat key input.
func newMarkdown(width int, dark bool) markdown {
	cfg := styles.LightStyleConfig
	if dark {
		cfg = styles.DarkStyleConfig
	}
	zero := uint(0)
	cfg.Document.Margin = &zero // the view owns the margins
	cfg.Document.BlockPrefix, cfg.Document.BlockSuffix = "", ""
	r, err := glamour.NewTermRenderer(glamour.WithStyles(cfg), glamour.WithWordWrap(max(width, 10)))
	if err != nil {
		return markdown{}
	}
	return markdown{r: r}
}

func (m markdown) render(text string, width int) string {
	if m.r != nil {
		if out, err := m.r.Render(text); err == nil {
			return strings.Trim(out, "\n")
		}
	}
	return lipgloss.NewStyle().Width(max(width, 10)).Render(text)
}

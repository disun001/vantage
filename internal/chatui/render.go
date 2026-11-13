package chatui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/disun001/vantage/internal/chat"
	"github.com/disun001/vantage/internal/store"
)

var (
	userStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	toolStyle = lipgloss.NewStyle().Faint(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dimStyle  = lipgloss.NewStyle().Faint(true)
)

// render turns stored events into transcript text wrapped to width. Session
// events feed the header, tool results and unknown lines stay out of the
// bare transcript (they are in the store for later slices).
func render(events []store.Event, width int) string {
	wrap := lipgloss.NewStyle().Width(max(width, 10))
	var parts []string
	for _, e := range events {
		switch e.Kind {
		case chat.KindUserMsg:
			parts = append(parts, wrap.Render(userStyle.Render("› ")+e.Text))
		case chat.KindAssistantMsg:
			parts = append(parts, wrap.Render(e.Text))
		case chat.KindToolUse:
			parts = append(parts, toolStyle.Render(oneLine("  ⚙ "+e.Text, width)))
		case chat.KindError:
			parts = append(parts, wrap.Render(errStyle.Render("! "+e.Text)))
		case chat.KindTurnEnd:
			if e.Text != "" { // set only when the turn failed
				parts = append(parts, wrap.Render(errStyle.Render("! "+e.Text)))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// oneLine cuts s to a single line of at most width cells.
func oneLine(s string, width int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	r := []rune(s)
	if n := max(width, 10); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// busy reports whether a turn is in flight: the last turn-level event is a
// user message with no turn_end or error after it.
func busy(events []store.Event) bool {
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Kind {
		case chat.KindUserMsg:
			return true
		case chat.KindTurnEnd, chat.KindError:
			return false
		}
	}
	return false
}

// sessionInfo returns the model and auth source of the latest session event.
func sessionInfo(events []store.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == chat.KindSession {
			return events[i].Text
		}
	}
	return ""
}

package chatui

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/disun001/vantage/internal/chat"
	"github.com/disun001/vantage/internal/store"
)

const maxResultLines = 8

var (
	userStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	toolStyle = lipgloss.NewStyle().Faint(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dimStyle  = lipgloss.NewStyle().Faint(true)
)

// render turns stored events into transcript text wrapped to width. Session
// events feed the header. Tool results show only when expanded; unknown lines
// stay in the store. Assistant text goes through md.
func render(events []store.Event, width int, md markdown, expand bool) string {
	wrap := lipgloss.NewStyle().Width(max(width, 10))
	var parts []string
	for _, e := range events {
		switch e.Kind {
		case chat.KindUserMsg:
			parts = append(parts, wrap.Render(userStyle.Render("› ")+e.Text))
		case chat.KindAssistantMsg:
			parts = append(parts, md.render(e.Text, width))
		case chat.KindToolResult:
			if expand {
				parts = append(parts, toolStyle.Render(resultLines(e.Text, width)))
			}
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

// resultLines indents a tool result under its call, at most maxResultLines.
func resultLines(text string, width int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > maxResultLines {
		more := len(lines) - maxResultLines
		lines = append(lines[:maxResultLines], "… "+strconv.Itoa(more)+" more lines")
	}
	for i, l := range lines {
		lines[i] = oneLine("    "+l, width)
	}
	return strings.Join(lines, "\n")
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

// usage is the context size of the latest turn and the output tokens summed
// over all turns, read from the result lines kept in raw.
func usage(events []store.Event) (ctxTokens, outTokens int64) {
	for _, e := range events {
		if e.Kind != chat.KindTurnEnd || !e.Raw.Valid {
			continue
		}
		var r struct {
			Usage map[string]any `json:"usage"`
		}
		if json.Unmarshal([]byte(e.Raw.String), &r) != nil {
			continue
		}
		ctxTokens = num(r.Usage, "input_tokens") + num(r.Usage, "cache_creation_input_tokens") + num(r.Usage, "cache_read_input_tokens")
		outTokens += num(r.Usage, "output_tokens")
	}
	return ctxTokens, outTokens
}

func num(m map[string]any, k string) int64 {
	f, _ := m[k].(float64)
	return int64(f)
}

// kilo formats a token count as 151 or 58.0k.
func kilo(n int64) string {
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}
	return strconv.FormatFloat(float64(n)/1000, 'f', 1, 64) + "k"
}

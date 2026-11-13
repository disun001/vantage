package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

const toolResultMax = 500 // runes kept in text; the full content stays in raw

// line is the subset of a stream-json line the mapper reads.
type line struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	APIKey    string `json:"apiKeySource"`
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
	ToolUseID string          `json:"tool_use_id"`
}

// ignoredSystem are system subtypes that carry no conversation. They are
// large (commands_changed is tens of KB) and repeat every turn.
var ignoredSystem = map[string]bool{
	"hook_started": true, "hook_response": true, "commands_changed": true,
}

// parseLine maps one stream-json line to zero or more events. The raw line
// is attached to the first event only. Unknown types are kept as KindUnknown,
// never dropped.
func parseLine(raw string) []Event {
	var l line
	if err := json.Unmarshal([]byte(raw), &l); err != nil {
		return []Event{{Kind: KindError, Text: "invalid json from claude", Raw: raw}}
	}
	var evs []Event
	switch l.Type {
	case "system":
		evs = mapSystem(l)
	case "assistant":
		evs = mapBlocks(l, KindAssistantMsg)
	case "user":
		evs = mapBlocks(l, KindToolResult)
	case "result":
		evs = []Event{{Kind: KindTurnEnd, Text: resultText(l), IsError: l.IsError}}
	case "rate_limit_event":
		return nil
	default:
		evs = []Event{{Kind: KindUnknown, Text: l.Type}}
	}
	if len(evs) > 0 {
		evs[0].Raw = raw
	}
	return evs
}

func resultText(l line) string {
	if l.IsError {
		return l.Result
	}
	return ""
}

func mapSystem(l line) []Event {
	switch {
	case l.Subtype == "init":
		return []Event{{Kind: KindSession, SessionID: l.SessionID, Model: l.Model, AuthSource: authSource(l.APIKey)}}
	case ignoredSystem[l.Subtype]:
		return nil
	default:
		return []Event{{Kind: KindUnknown, Text: "system " + l.Subtype}}
	}
}

// authSource names where claude says its credentials came from. "none" means
// no API key was used, so the logged-in subscription is.
func authSource(s string) string {
	switch s {
	case "none", "":
		return "subscription"
	case "ANTHROPIC_API_KEY":
		return "api key"
	default:
		return s
	}
}

// mapBlocks turns assistant text and tool_use blocks, or user tool_result
// blocks, into events. Thinking and other block types are skipped.
func mapBlocks(l line, textKind string) []Event {
	var blocks []block
	if err := json.Unmarshal(l.Message.Content, &blocks); err != nil {
		return nil // content may be a plain string (an echoed user message)
	}
	var evs []Event
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if textKind == KindAssistantMsg && strings.TrimSpace(b.Text) != "" {
				evs = append(evs, Event{Kind: KindAssistantMsg, Text: b.Text})
			}
		case "tool_use":
			evs = append(evs, Event{Kind: KindToolUse, Text: toolSummary(b)})
		case "tool_result":
			evs = append(evs, Event{Kind: KindToolResult, Text: truncate(toolResultContent(b.Content), toolResultMax)})
		}
	}
	return evs
}

// toolSummary is "Name arg" where arg is the first short string input
// (file_path, command, pattern ...), enough for a one-line marker.
func toolSummary(b block) string {
	var in map[string]any
	if err := json.Unmarshal(b.Input, &in); err != nil {
		return b.Name
	}
	for _, k := range []string{"file_path", "command", "pattern", "path", "url", "query"} {
		if s, ok := in[k].(string); ok && s != "" {
			return fmt.Sprintf("%s %s", b.Name, truncate(s, 120))
		}
	}
	return b.Name
}

// toolResultContent flattens a tool_result's content, which is either a
// string or a list of {type:"text",text} blocks.
func toolResultContent(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []block
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p.Text)
	}
	return sb.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Package chat is the chat core: a Provider seam, the claude stream-json
// child behind it, and the glue that persists a conversation to the store.
// It imports no UI and no tmux, so a pane, a daemon or a test can drive it.
package chat

// Event kinds. They are stored in events.kind (see docs/chat.md).
const (
	KindUserMsg      = "user_msg"
	KindSession      = "session"
	KindAssistantMsg = "assistant_msg"
	KindToolUse      = "tool_use"
	KindToolResult   = "tool_result"
	KindTurnEnd      = "turn_end"
	KindError        = "error"
	KindUnknown      = "unknown"
)

// Event is one normalized provider event.
type Event struct {
	Kind string
	Text string
	Raw  string // original provider line; set on the first event of a line

	// Set on KindSession.
	SessionID  string
	Model      string
	AuthSource string // "subscription", "api key" or the provider's own word

	// Set on KindTurnEnd.
	IsError bool
}

// StartOpts says how to start a provider session.
type StartOpts struct {
	Cwd      string
	ResumeID string // provider session id to continue; empty starts fresh
}

// Provider starts conversations with one AI backend.
type Provider interface {
	Name() string
	Start(StartOpts) (Session, error)
}

// Session is one running conversation process.
type Session interface {
	// Send submits a user message.
	Send(text string) error
	// Events delivers normalized events and is closed when the session ends.
	Events() <-chan Event
	// Close stops the session. It is safe to call more than once.
	Close() error
}

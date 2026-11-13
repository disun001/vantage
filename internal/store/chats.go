package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/disun001/vantage/internal/store/gen"
)

// Chat statuses (chats.status).
const (
	StatusIdle        = "idle"
	StatusRunning     = "running"
	StatusError       = "error"
	StatusInterrupted = "interrupted"
)

// ErrNotFound means no chat has the given id.
var ErrNotFound = errors.New("chat not found")

// NewChat is the input to CreateChat.
type NewChat struct {
	ID, Provider, Cwd string
	Session, Pane     string // tmux session name and pane id; may be empty
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func now() int64 { return time.Now().UnixMilli() }

// CreateChat inserts an idle chat.
func (s *Store) CreateChat(ctx context.Context, n NewChat) error {
	t := now()
	err := s.q.CreateChat(ctx, gen.CreateChatParams{
		ID: n.ID, Provider: n.Provider, SessionName: nullStr(n.Session),
		PaneID: nullStr(n.Pane), Cwd: n.Cwd, CreatedAt: t, UpdatedAt: t,
	})
	if err != nil {
		return fmt.Errorf("create chat %q: %w", n.ID, err)
	}
	return nil
}

// Chat returns one chat, or ErrNotFound.
func (s *Store) Chat(ctx context.Context, id string) (Chat, error) {
	c, err := s.q.GetChat(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Chat{}, fmt.Errorf("chat %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return Chat{}, fmt.Errorf("get chat %q: %w", id, err)
	}
	return c, nil
}

// Chats lists chats, most recently updated first.
func (s *Store) Chats(ctx context.Context) ([]Chat, error) {
	cs, err := s.q.ListChats(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	return cs, nil
}

// SetStatus updates a chat's status.
func (s *Store) SetStatus(ctx context.Context, id, status string) error {
	if err := s.q.SetChatStatus(ctx, gen.SetChatStatusParams{Status: status, UpdatedAt: now(), ID: id}); err != nil {
		return fmt.Errorf("set status of %q: %w", id, err)
	}
	return nil
}

// SetProviderSession saves the provider's session id, used for resume.
func (s *Store) SetProviderSession(ctx context.Context, id, session string) error {
	err := s.q.SetChatSession(ctx, gen.SetChatSessionParams{ProviderSession: nullStr(session), UpdatedAt: now(), ID: id})
	if err != nil {
		return fmt.Errorf("set session of %q: %w", id, err)
	}
	return nil
}

// SetPane points a chat at the tmux pane it now lives in.
func (s *Store) SetPane(ctx context.Context, id, session, pane string) error {
	err := s.q.SetChatPane(ctx, gen.SetChatPaneParams{SessionName: nullStr(session), PaneID: nullStr(pane), UpdatedAt: now(), ID: id})
	if err != nil {
		return fmt.Errorf("set pane of %q: %w", id, err)
	}
	return nil
}

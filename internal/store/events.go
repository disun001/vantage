package store

import (
	"context"
	"fmt"

	"github.com/disun001/vantage/internal/store/gen"
)

// NewEvent is the input to Append. Status, when non-empty, is written to the
// chat in the same transaction so readers never see an event without the
// status that goes with it.
type NewEvent struct {
	ChatID string
	Turn   int64
	Kind   string
	Text   string
	Raw    string // original provider line; may be empty
	Status string
}

// Append adds an event at the next seq of its chat.
func (s *Store) Append(ctx context.Context, e NewEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	t := now()
	err = q.AppendEvent(ctx, gen.AppendEventParams{
		ChatID: e.ChatID, Turn: e.Turn, Ts: t, Kind: e.Kind, Text: e.Text, Raw: nullStr(e.Raw),
	})
	if err != nil {
		return fmt.Errorf("append %s event to %q: %w", e.Kind, e.ChatID, err)
	}
	if e.Status != "" {
		err = q.SetChatStatus(ctx, gen.SetChatStatusParams{Status: e.Status, UpdatedAt: t, ID: e.ChatID})
		if err != nil {
			return fmt.Errorf("set status of %q: %w", e.ChatID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// Events returns a chat's events in order.
func (s *Store) Events(ctx context.Context, chatID string) ([]Event, error) {
	es, err := s.q.ListEvents(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("list events of %q: %w", chatID, err)
	}
	return es, nil
}

// LastTurn returns the highest turn number used by a chat (0 if none).
func (s *Store) LastTurn(ctx context.Context, chatID string) (int64, error) {
	n, err := s.q.LastTurn(ctx, chatID)
	if err != nil {
		return 0, fmt.Errorf("last turn of %q: %w", chatID, err)
	}
	return n, nil
}

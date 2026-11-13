package chat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/disun001/vantage/internal/store"
)

// Options configures a Chat.
type Options struct {
	// TurnTimeout aborts a turn that has not ended after this long. Zero
	// means no timeout.
	TurnTimeout time.Duration
	// OnEvent is called, in order, for every event after it is stored. It
	// runs on the Chat's own goroutine, so it must not block for long.
	OnEvent func(store.Event)
}

// Chat is one persisted conversation: a store row plus a lazily started
// provider session. The child is started on the first Send and restarted
// with the saved session id whenever it has died.
type Chat struct {
	st   *store.Store
	p    Provider
	id   string
	opts Options

	mu    sync.Mutex
	cwd   string
	resID string
	turn  int64
	sess  Session
	pump  chan struct{} // closed when the current session's pump has finished
	timer *time.Timer
}

// Open loads chat id. A chat found "running" had its process die mid-turn, so
// that turn is marked interrupted before anything else happens.
func Open(ctx context.Context, st *store.Store, p Provider, id string, opts Options) (*Chat, error) {
	row, err := st.Chat(ctx, id)
	if err != nil {
		return nil, err
	}
	turn, err := st.LastTurn(ctx, id)
	if err != nil {
		return nil, err
	}
	c := &Chat{st: st, p: p, id: id, opts: opts, cwd: row.Cwd, resID: row.ProviderSession.String, turn: turn}
	if row.Status == store.StatusRunning {
		err := c.record(ctx, store.NewEvent{
			ChatID: id, Turn: turn, Kind: KindError, Status: store.StatusInterrupted,
			Text: "turn interrupted: the previous process ended before the turn finished",
		})
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// ID returns the chat id.
func (c *Chat) ID() string { return c.id }

// Send stores the user message, marks the chat running and hands the message
// to the provider, starting or resuming the child if needed.
func (c *Chat) Send(ctx context.Context, text string) error {
	c.mu.Lock()
	c.turn++
	turn := c.turn
	c.mu.Unlock()
	err := c.record(ctx, store.NewEvent{ChatID: c.id, Turn: turn, Kind: KindUserMsg, Text: text, Status: store.StatusRunning})
	if err != nil {
		return err
	}
	if err := c.deliver(text); err != nil {
		c.fail(ctx, turn, err.Error())
		return err
	}
	c.armTimeout(turn)
	return nil
}

// deliver sends to the live session, respawning once if the child is gone.
func (c *Chat) deliver(text string) error {
	for attempt := 0; ; attempt++ {
		sess, err := c.session()
		if err != nil {
			return err
		}
		if err = sess.Send(text); err == nil {
			return nil
		}
		c.drop(sess)
		if attempt > 0 {
			return err
		}
	}
}

// session returns the live session, starting one (resuming if a session id
// is known) when there is none.
func (c *Chat) session() (Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sess != nil {
		return c.sess, nil
	}
	s, err := c.p.Start(StartOpts{Cwd: c.cwd, ResumeID: c.resID})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", c.p.Name(), err)
	}
	c.sess = s
	c.pump = make(chan struct{})
	go c.run(s, c.pump)
	return s, nil
}

// drop forgets sess if it is still the current one.
func (c *Chat) drop(sess Session) {
	c.mu.Lock()
	if c.sess == sess {
		c.sess = nil
	}
	c.mu.Unlock()
	_ = sess.Close()
}

// run stores a session's events until it ends.
func (c *Chat) run(s Session, done chan struct{}) {
	defer close(done)
	ctx := context.Background()
	for ev := range s.Events() {
		c.mu.Lock()
		turn := c.turn
		c.mu.Unlock()
		c.handle(ctx, turn, ev)
	}
	c.mu.Lock()
	if c.sess == s {
		c.sess = nil // the child ended by itself; the next Send resumes
	}
	c.mu.Unlock()
}

func (c *Chat) handle(ctx context.Context, turn int64, ev Event) {
	ne := store.NewEvent{ChatID: c.id, Turn: turn, Kind: ev.Kind, Text: ev.Text, Raw: ev.Raw}
	switch ev.Kind {
	case KindSession:
		c.mu.Lock()
		c.resID = ev.SessionID
		c.mu.Unlock()
		ne.Text = fmt.Sprintf("model=%s auth=%s", ev.Model, ev.AuthSource)
		if err := c.st.SetProviderSession(ctx, c.id, ev.SessionID); err != nil {
			ne.Text += " (session id not saved: " + err.Error() + ")"
		}
	case KindTurnEnd:
		c.stopTimer()
		ne.Status = store.StatusIdle
		if ev.IsError {
			ne.Status = store.StatusError
		}
	case KindError:
		c.stopTimer()
		ne.Status = store.StatusError
	}
	_ = c.record(ctx, ne) // a store failure here has nowhere useful to go; the UI reads the db
}

// record appends an event and notifies OnEvent.
func (c *Chat) record(ctx context.Context, ne store.NewEvent) error {
	if err := c.st.Append(ctx, ne); err != nil {
		return err
	}
	if c.opts.OnEvent == nil {
		return nil
	}
	es, err := c.st.Events(ctx, c.id)
	if err != nil || len(es) == 0 {
		return err
	}
	c.opts.OnEvent(es[len(es)-1])
	return nil
}

// Interrupt kills the child and marks the turn interrupted. The next Send
// resumes the conversation in a fresh child.
func (c *Chat) Interrupt(ctx context.Context) error {
	return c.abort(ctx, "turn interrupted by user", store.StatusInterrupted)
}

func (c *Chat) abort(ctx context.Context, reason, status string) error {
	c.stopTimer()
	c.mu.Lock()
	s, pump, turn := c.sess, c.pump, c.turn
	c.sess = nil
	c.mu.Unlock()
	if s != nil {
		_ = s.Close()
		<-pump // flush events already read before recording the abort
	}
	return c.record(ctx, store.NewEvent{ChatID: c.id, Turn: turn, Kind: KindError, Text: reason, Status: status})
}

// Close stops the child. The chat stays resumable.
func (c *Chat) Close() error {
	c.stopTimer()
	c.mu.Lock()
	s, pump := c.sess, c.pump
	c.sess = nil
	c.mu.Unlock()
	if s == nil {
		return nil
	}
	err := s.Close()
	<-pump
	return err
}

func (c *Chat) fail(ctx context.Context, turn int64, reason string) {
	_ = c.record(ctx, store.NewEvent{ChatID: c.id, Turn: turn, Kind: KindError, Text: reason, Status: store.StatusError})
}

func (c *Chat) armTimeout(turn int64) {
	if c.opts.TurnTimeout <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(c.opts.TurnTimeout, func() {
		c.mu.Lock()
		current := c.turn == turn
		c.mu.Unlock()
		if current {
			_ = c.abort(context.Background(), fmt.Sprintf("turn timed out after %s", c.opts.TurnTimeout), store.StatusError)
		}
	})
}

func (c *Chat) stopTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

// ErrNotFound is returned by Open for an unknown chat id.
var ErrNotFound = store.ErrNotFound

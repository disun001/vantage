package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/disun001/vantage/internal/store"
)

type env struct {
	st  *store.Store
	log string
	dir string
}

func setup(t *testing.T, scenario string) (*env, Claude) {
	t.Helper()
	if _, err := os.Stat("testdata/fake-claude.sh"); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	tmp := t.TempDir()
	e := &env{st: st, log: filepath.Join(tmp, "spawns.log"), dir: tmp}
	abs, _ := filepath.Abs("testdata/fake-claude.sh")
	return e, Claude{
		Exe: abs,
		Env: []string{"PATH=" + os.Getenv("PATH"), "FAKE_CLAUDE_SCENARIO=" + scenario, "FAKE_CLAUDE_LOG=" + e.log},
	}
}

func newChat(t *testing.T, e *env, p Provider, opts Options) *Chat {
	t.Helper()
	ctx := context.Background()
	if err := e.st.CreateChat(ctx, store.NewChat{ID: "c1", Provider: "claude", Cwd: e.dir}); err != nil {
		t.Fatal(err)
	}
	c, err := Open(ctx, e.st, p, "c1", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func waitStatus(t *testing.T, e *env, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := e.st.Chat(context.Background(), "c1"); err == nil && c.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	c, _ := e.st.Chat(context.Background(), "c1")
	t.Fatalf("status %q, want %q; events: %v", c.Status, want, kinds(t, e))
}

func kinds(t *testing.T, e *env) []string {
	t.Helper()
	es, err := e.st.Events(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, ev := range es {
		out = append(out, ev.Kind)
	}
	return out
}

func spawns(t *testing.T, e *env) []string {
	t.Helper()
	b, _ := os.ReadFile(e.log)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestNormalTurn(t *testing.T) {
	e, p := setup(t, "normal")
	var seen []string
	c := newChat(t, e, p, Options{OnEvent: func(ev store.Event) { seen = append(seen, ev.Kind) }})
	if err := c.Send(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, e, store.StatusIdle)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(kinds(t, e), ",")
	if got != "user_msg,session,assistant_msg,turn_end" {
		t.Fatalf("kinds %s", got)
	}
	row, _ := e.st.Chat(context.Background(), "c1")
	if row.ProviderSession.String == "" {
		t.Fatal("session id not saved")
	}
	if len(seen) != 4 {
		t.Fatalf("OnEvent saw %v", seen)
	}
}

func TestToolUseTurn(t *testing.T) {
	e, p := setup(t, "tooluse")
	c := newChat(t, e, p, Options{})
	if err := c.Send(context.Background(), "read it"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, e, store.StatusIdle)
	got := strings.Join(kinds(t, e), ",")
	if got != "user_msg,session,tool_use,tool_result,assistant_msg,turn_end" {
		t.Fatalf("kinds %s", got)
	}
}

func TestSecondTurnReusesChild(t *testing.T) {
	e, p := setup(t, "normal")
	c := newChat(t, e, p, Options{})
	for i := 0; i < 2; i++ {
		if err := c.Send(context.Background(), "hi"); err != nil {
			t.Fatal(err)
		}
		waitUntilTurnEnds(t, e, i+1)
	}
	if n := len(spawns(t, e)); n != 1 {
		t.Fatalf("spawned %d children, want 1", n)
	}
	if n, _ := e.st.LastTurn(context.Background(), "c1"); n != 2 {
		t.Fatalf("turn %d", n)
	}
}

func waitUntilTurnEnds(t *testing.T, e *env, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		count := 0
		for _, k := range kinds(t, e) {
			if k == KindTurnEnd {
				count++
			}
		}
		if count >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("turn %d never ended: %v", n, kinds(t, e))
}

func TestCrashMidTurnThenResume(t *testing.T) {
	e, p := setup(t, "crash")
	c := newChat(t, e, p, Options{})
	if err := c.Send(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, e, store.StatusError)
	es, _ := e.st.Events(context.Background(), "c1")
	last := es[len(es)-1]
	if last.Kind != KindError || !strings.Contains(last.Text, "exited") {
		t.Fatalf("last event %+v", last)
	}
	if err := c.Send(context.Background(), "again"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, e, store.StatusIdle)
	sp := spawns(t, e)
	if len(sp) != 2 || strings.Contains(sp[0], "--resume") || !strings.Contains(sp[1], "--resume 00000000-0000-4000-8000-000000000001") {
		t.Fatalf("spawns %q", sp)
	}
}

func TestBadJSONLineDoesNotEndTurn(t *testing.T) {
	e, p := setup(t, "badjson")
	c := newChat(t, e, p, Options{})
	if err := c.Send(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	waitUntilTurnEnds(t, e, 1)
	got := strings.Join(kinds(t, e), ",")
	if got != "user_msg,session,error,assistant_msg,turn_end" {
		t.Fatalf("kinds %s", got)
	}
	waitStatus(t, e, store.StatusIdle)
}

func TestHangTimesOut(t *testing.T) {
	e, p := setup(t, "hang")
	c := newChat(t, e, p, Options{TurnTimeout: 200 * time.Millisecond})
	if err := c.Send(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, e, store.StatusError)
	es, _ := e.st.Events(context.Background(), "c1")
	if last := es[len(es)-1]; !strings.Contains(last.Text, "timed out") {
		t.Fatalf("last %+v", last)
	}
}

func TestInterrupt(t *testing.T) {
	e, p := setup(t, "hang")
	c := newChat(t, e, p, Options{})
	if err := c.Send(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if err := c.Interrupt(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, e, store.StatusInterrupted)
}

func TestOpenMarksOrphanRunningInterrupted(t *testing.T) {
	e, p := setup(t, "normal")
	ctx := context.Background()
	if err := e.st.CreateChat(ctx, store.NewChat{ID: "c1", Provider: "claude", Cwd: e.dir}); err != nil {
		t.Fatal(err)
	}
	if err := e.st.Append(ctx, store.NewEvent{ChatID: "c1", Turn: 1, Kind: KindUserMsg, Text: "hi", Status: store.StatusRunning}); err != nil {
		t.Fatal(err)
	}
	c, err := Open(ctx, e.st, p, "c1", Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	row, _ := e.st.Chat(ctx, "c1")
	if row.Status != store.StatusInterrupted {
		t.Fatalf("status %q", row.Status)
	}
	if got := strings.Join(kinds(t, e), ","); got != "user_msg,error" {
		t.Fatalf("kinds %s", got)
	}
}

func TestStartFailure(t *testing.T) {
	e, _ := setup(t, "normal")
	c := newChat(t, e, Claude{Exe: filepath.Join(e.dir, "missing")}, Options{})
	if err := c.Send(context.Background(), "hi"); err == nil {
		t.Fatal("want error")
	}
	waitStatus(t, e, store.StatusError)
}

func TestOpenUnknownChat(t *testing.T) {
	e, p := setup(t, "normal")
	if _, err := Open(context.Background(), e.st, p, "nope", Options{}); err == nil {
		t.Fatal("want error")
	}
}

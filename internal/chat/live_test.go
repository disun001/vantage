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

// TestLiveClaude runs the real claude CLI, which spends subscription quota.
// It only runs with VANTAGE_LIVE_CLAUDE=1; run it by hand before releases.
func TestLiveClaude(t *testing.T) {
	if os.Getenv("VANTAGE_LIVE_CLAUDE") != "1" {
		t.Skip("set VANTAGE_LIVE_CLAUDE=1 to run against the real claude")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.CreateChat(ctx, store.NewChat{ID: "live", Provider: "claude", Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	c, err := Open(ctx, st, Claude{Env: os.Environ()}, "live", Options{TurnTimeout: 2 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if err := c.Send(ctx, "Reply with exactly the word: pong"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if row, _ := st.Chat(ctx, "live"); row.Status == store.StatusIdle {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	es, _ := st.Events(ctx, "live")
	var reply string
	for _, e := range es {
		if e.Kind == KindAssistantMsg {
			reply += e.Text
		}
	}
	if !strings.Contains(strings.ToLower(reply), "pong") {
		t.Fatalf("reply %q; events %+v", reply, es)
	}
}

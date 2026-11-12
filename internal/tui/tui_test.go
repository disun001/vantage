package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/tmux"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func testClient(t *testing.T) tmux.Client {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	c := tmux.Client{Socket: fmt.Sprintf("vantage-test-%d-%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() { _ = c.KillServer() })
	return c
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestEmbeddedSession runs the whole program against a throwaway tmux server:
// the sidebar lists the session, the embedded client shows its shell, typed
// keys reach it, and quitting leaves the session running.
func TestEmbeddedSession(t *testing.T) {
	c := testClient(t)
	if _, err := c.NewSession("alpha", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	pr, pw := io.Pipe()
	out := &syncBuf{}
	h := &hub{}
	p := tea.NewProgram(newModel(Config{Client: c}, h), tea.WithInput(pr), tea.WithOutput(out), tea.WithAltScreen())
	h.send = p.Send
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	t.Cleanup(func() { p.Kill(); _ = pw.Close() })

	p.Send(tea.WindowSizeMsg{Width: 100, Height: 24})
	waitFor(t, "sidebar row", func() bool { return strings.Contains(out.String(), "alpha") })

	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}) // focus the embedded shell
	for _, r := range "echo vt$((20+22))" {
		p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	p.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitFor(t, "shell output", func() bool { return strings.Contains(out.String(), "vt42") })

	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s"), Alt: true})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("q in the sidebar did not quit")
	}
	if !c.HasSession("alpha") {
		t.Error("quitting the TUI must leave the tmux session running")
	}
}

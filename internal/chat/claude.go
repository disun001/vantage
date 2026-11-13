package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Claude drives the `claude` CLI headless, one long-lived child per session.
type Claude struct {
	// Exe is the claude executable. Empty means "claude" from PATH.
	Exe string
	// Args are extra arguments placed before the fixed ones (tests use them).
	Args []string
	// Env is the child's environment, normally os.Environ() from the edge.
	Env []string
	// AllowAPIKey keeps ANTHROPIC_API_KEY in the child's environment. By
	// default it is removed: `-p` prefers an exported key and would bill API
	// credits instead of the subscription.
	AllowAPIKey bool
}

// Name implements Provider.
func (c Claude) Name() string { return "claude" }

// childEnv drops ANTHROPIC_API_KEY unless allowed.
func (c Claude) childEnv() []string {
	if c.AllowAPIKey {
		return c.Env
	}
	out := make([]string, 0, len(c.Env))
	for _, kv := range c.Env {
		if !strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") {
			out = append(out, kv)
		}
	}
	return out
}

// claudeArgs are the fixed flags. Plan mode is read-only. Never add --bare:
// it skips the subscription login.
func claudeArgs(extra []string, resume string) []string {
	args := append([]string{}, extra...)
	args = append(args, "-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "plan")
	if resume != "" {
		args = append(args, "--resume", resume)
	}
	return args
}

// Start implements Provider.
func (c Claude) Start(o StartOpts) (Session, error) {
	exe := c.Exe
	if exe == "" {
		exe = "claude"
	}
	cmd := exec.Command(exe, claudeArgs(c.Args, o.ResumeID)...)
	cmd.Dir = o.Cwd
	cmd.Env = c.childEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("claude stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claude stdout: %w", err)
	}
	var stderr tail
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", exe, err)
	}
	s := &claudeSession{cmd: cmd, stdin: stdin, events: make(chan Event, 64), done: make(chan struct{}), stderr: &stderr}
	go s.read(stdout)
	return s, nil
}

type claudeSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	events chan Event
	stderr *tail

	mu      sync.Mutex
	closing bool
	done    chan struct{} // closed by Close to unblock the reader
	once    sync.Once
}

func (s *claudeSession) Events() <-chan Event { return s.events }

// Send writes one stream-json user message.
func (s *claudeSession) Send(text string) error {
	msg, err := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": text},
	})
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return fmt.Errorf("send: session closed")
	}
	if _, err := s.stdin.Write(append(msg, '\n')); err != nil {
		return fmt.Errorf("send to claude: %w", err)
	}
	return nil
}

// Close kills the child and waits for the reader to finish.
func (s *claudeSession) Close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closing = true
		s.mu.Unlock()
		close(s.done)
		_ = s.stdin.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	})
	for range s.events { // wait for the reader to close the channel
	}
	return nil
}

// read maps stdout lines to events, then reports an unexpected exit.
func (s *claudeSession) read(stdout io.Reader) {
	defer close(s.events)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // init and tool lines can be large
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		for _, ev := range parseLine(raw) {
			if !s.emit(ev) {
				_ = s.cmd.Wait()
				return
			}
		}
	}
	scanErr := sc.Err()
	waitErr := s.cmd.Wait()
	s.mu.Lock()
	closing := s.closing
	s.mu.Unlock()
	if closing {
		return
	}
	s.emit(Event{Kind: KindError, Text: exitReason(scanErr, waitErr, s.stderr.String())})
}

func (s *claudeSession) emit(ev Event) bool {
	select {
	case s.events <- ev:
		return true
	case <-s.done:
		return false
	}
}

func exitReason(scanErr, waitErr error, stderr string) string {
	reason := "claude exited"
	switch {
	case scanErr != nil:
		reason = fmt.Sprintf("claude output: %v", scanErr)
	case waitErr != nil:
		reason = fmt.Sprintf("claude exited: %v", waitErr)
	}
	if stderr = strings.TrimSpace(stderr); stderr != "" {
		reason += ": " + stderr
	}
	return reason
}

// tail keeps the last few KB written to it (claude's stderr).
type tail struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

const tailMax = 4096

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf.Write(p)
	if over := t.buf.Len() - tailMax; over > 0 {
		t.buf.Next(over)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

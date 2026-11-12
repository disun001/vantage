// Package embed runs a program (a tmux client) in a PTY and keeps a virtual
// screen of its output, so a TUI can draw it inside a region.
//
// It owns the PTY and the emulator only. It knows nothing about tmux or
// layout; callers pass the command to run.
package embed

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Term is a child process attached to a PTY and mirrored in an emulator.
type Term struct {
	cmd  *exec.Cmd
	pty  *os.File
	mu   sync.Mutex // guards em: reads (Render) race with the pump's writes
	em   *vt.Emulator
	done chan struct{}
}

// Start runs cmd on a new PTY of the given size. onOutput is called (from a
// goroutine) after the screen changed; onExit once when the process ends.
func Start(cmd *exec.Cmd, cols, rows int, onOutput, onExit func()) (*Term, error) {
	if cols < 1 || rows < 1 {
		return nil, fmt.Errorf("embed: bad size %dx%d", cols, rows)
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", cmd.Path, err)
	}
	t := &Term{cmd: cmd, pty: f, em: vt.NewEmulator(cols, rows), done: make(chan struct{})}

	// The emulator answers terminal queries (cursor position, device
	// attributes) on its read side. Forward them to the child, or the child
	// blocks waiting for a reply.
	go func() { _, _ = io.Copy(f, t.em) }()

	go func() {
		defer close(t.done)
		buf := make([]byte, 32*1024)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				t.mu.Lock()
				_, _ = t.em.Write(buf[:n])
				t.mu.Unlock()
				if onOutput != nil {
					onOutput()
				}
			}
			if err != nil {
				break
			}
		}
		_ = cmd.Wait()
		if onExit != nil {
			onExit()
		}
	}()
	return t, nil
}

// Write sends raw bytes (keys, pasted text) to the child.
func (t *Term) Write(p []byte) error {
	_, err := t.pty.Write(p)
	return err
}

// Paste sends text as a paste: bracketed when the child asked for it.
func (t *Term) Paste(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.em.Paste(text)
}

// Mouse sends a mouse event, encoded the way the child asked for. It is a
// no-op while the child has not enabled mouse reporting.
func (t *Term) Mouse(m uv.MouseEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.em.SendMouse(m)
}

// PID is the child's process id.
func (t *Term) PID() int { return t.cmd.Process.Pid }

// Resize changes the PTY and the emulator size.
func (t *Term) Resize(cols, rows int) {
	if cols < 1 || rows < 1 {
		return
	}
	_ = pty.Setsize(t.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	t.mu.Lock()
	t.em.Resize(cols, rows)
	t.mu.Unlock()
}

// Lines returns the screen as one ANSI-styled string per row. When
// showCursor is true the cursor cell is drawn reversed, because Bubble Tea
// cannot place the real cursor inside a region.
func (t *Term) Lines(showCursor bool) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var restore func()
	if showCursor && !t.em.CursorHidden() {
		restore = t.reverseCursor()
	}
	out := splitLines(t.em.Render())
	if restore != nil {
		restore()
	}
	return out
}

// reverseCursor flips the reverse attribute on the cursor cell and returns a
// func that puts the cell back. Caller holds t.mu.
func (t *Term) reverseCursor() func() {
	p := t.em.CursorPosition()
	c := t.em.CellAt(p.X, p.Y)
	if c == nil {
		return nil
	}
	orig := c.Clone()
	n := c.Clone()
	if n.Content == "" {
		n.Content, n.Width = " ", 1
	}
	n.Style.Attrs ^= uv.AttrReverse
	t.em.SetCell(p.X, p.Y, n)
	return func() { t.em.SetCell(p.X, p.Y, orig) }
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// Done is closed when the child has exited and its output is drained.
func (t *Term) Done() <-chan struct{} { return t.done }

// Close stops the child and releases the PTY.
func (t *Term) Close() {
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	_ = t.pty.Close()
	_ = t.em.Close()
}

package askpass

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// TTY is the controlling terminal opened as /dev/tty.
type TTY struct {
	f *os.File
	r *bufio.Reader
}

// OpenTTY opens /dev/tty.
func OpenTTY() (*TTY, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("no terminal to ask on: %w", err)
	}
	return &TTY{f: f, r: bufio.NewReader(f)}, nil
}

// Close releases the terminal.
func (t *TTY) Close() { _ = t.f.Close() }

// Show prints text.
func (t *TTY) Show(text string) { _, _ = t.f.WriteString(text) } // nothing to do if the terminal is gone

// ReadLine reads an echoed line.
func (t *TTY) ReadLine() (string, error) {
	s, err := t.r.ReadString('\n')
	return strings.TrimRight(s, "\r\n"), err
}

// ReadSecret reads a line without echo.
func (t *TTY) ReadSecret() (string, error) {
	b, err := term.ReadPassword(int(t.f.Fd()))
	return string(b), err
}

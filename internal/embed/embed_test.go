package embed

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestStartMirrorsOutput(t *testing.T) {
	exited := make(chan struct{})
	term, err := Start(exec.Command("sh", "-c", "printf 'hello\\n'; cat"), 20, 4, nil, func() { close(exited) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)

	waitFor(t, func() bool { return strings.Contains(strings.Join(term.Lines(false), "\n"), "hello") })

	if err := term.Write([]byte("world\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return strings.Contains(strings.Join(term.Lines(true), "\n"), "world") })

	term.Resize(30, 6)
	if got := len(term.Lines(false)); got != 6 {
		t.Errorf("rows after resize = %d, want 6", got)
	}

	term.Close()
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("onExit not called")
	}
}

func TestStartRejectsBadSize(t *testing.T) {
	if _, err := Start(exec.Command("true"), 0, 5, nil, nil); err == nil {
		t.Fatal("want error for zero width")
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

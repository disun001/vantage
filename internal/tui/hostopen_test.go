package tui

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/capture"
	"github.com/disun001/vantage/internal/hostuse"
	"github.com/disun001/vantage/internal/tmux"
)

func panesAt(refs ...tmux.PaneRef) *fakeUse { return &fakeUse{use: hostuse.Usage{Panes: refs}} }

var (
	paneA = tmux.PaneRef{Session: "proj", Window: 1, WindowName: "build", Pane: "%3"}
	paneB = tmux.PaneRef{Session: "ops", Window: 2, WindowName: "logs", Pane: "%7"}
)

func pressEnterOnWeb(t *testing.T, u *fakeUse) model {
	t.Helper()
	m, _ := usedHost(t, u)
	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("Enter on a host did nothing")
	}
	return settle(m, cmd)
}

func TestEnterOnASingleLivePaneGoesThere(t *testing.T) {
	m, _ := usedHost(t, panesAt(paneA))
	m, cmd := press(m, "enter")
	next, cmd := m.Update(cmd())
	if next.(model).hostPick != nil {
		t.Fatal("one pane needs no choice")
	}
	msg, ok := cmd().(hostGoMsg)
	if !ok || msg.session != "proj" || msg.err != nil {
		t.Errorf("msg = %+v", msg)
	}
}

func TestEnterOnSeveralLivePanesAsksWhich(t *testing.T) {
	m := pressEnterOnWeb(t, panesAt(paneA, paneB))
	m.w, m.h = 80, 24
	if m.hostPick == nil || m.hostPick.alias != "web" {
		t.Fatalf("pick = %+v", m.hostPick)
	}
	box := popupText(m)
	for _, want := range []string{"connected to web", "1 proj:1 build", "2 ops:2 logs", "esc cancels"} {
		if !strings.Contains(box, want) {
			t.Errorf("pop-up lacks %q:\n%s", want, box)
		}
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("height %d", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("line %d is %d cells wide", i, w)
		}
	}
	m, _ = press(m, "j")
	m, cmd := press(m, "enter")
	if m.hostPick != nil || cmd == nil {
		t.Fatal("Enter must close the choice and go")
	}
	if msg := cmd().(hostGoMsg); msg.session != "ops" {
		t.Errorf("went to %q, want ops", msg.session)
	}
}

func TestHostPickKeys(t *testing.T) {
	m := pressEnterOnWeb(t, panesAt(paneA, paneB))
	m, _ = press(m, "k") // wraps to the last
	if m.hostPick.cur != 1 {
		t.Errorf("cur = %d", m.hostPick.cur)
	}
	m, _ = press(m, "j")
	if m.hostPick.cur != 0 {
		t.Errorf("cur = %d", m.hostPick.cur)
	}
	m, cmd := press(m, "2")
	if m.hostPick != nil || cmd == nil || cmd().(hostGoMsg).session != "ops" {
		t.Error("2 must pick the second pane")
	}
	m = pressEnterOnWeb(t, panesAt(paneA, paneB))
	m, cmd = press(m, "9") // no ninth pane
	if m.hostPick == nil || cmd != nil {
		t.Error("an unused digit must change nothing")
	}
	m, cmd = press(m, "esc")
	if m.hostPick != nil || cmd != nil {
		t.Error("esc must cancel")
	}
}

func TestEnterWithNoLivePaneNeedsAnOpenWorkspace(t *testing.T) {
	m := pressEnterOnWeb(t, panesAt())
	if m.notice != "open a workspace first" || m.hostPick != nil {
		t.Errorf("notice %q, pick %v", m.notice, m.hostPick)
	}
}

func TestEnterReportsAFailedLookup(t *testing.T) {
	m := pressEnterOnWeb(t, &fakeUse{findErr: io.ErrUnexpectedEOF})
	if !strings.Contains(m.notice, "could not look for open panes") {
		t.Errorf("notice %q", m.notice)
	}
}

func TestMouseIgnoredWhilePickOpen(t *testing.T) {
	m := pressEnterOnWeb(t, panesAt(paneA, paneB))
	before := m.hostCur
	next, _ := m.Update(tea.MouseMsg{X: 3, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := next.(model); got.hostCur != before || got.hostPick == nil {
		t.Error("the mouse reached past the choice")
	}
}

// TestEnterOnAHostEndToEnd runs the program on a throwaway tmux server with a
// stand-in ssh: with no open pane Enter opens a window running ssh, with one it
// shows that pane's session and window.
func TestEnterOnAHostEndToEnd(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"fake ssh $*\" > \"" + filepath.Join(bin, "args") + "\"\nsleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	c := testClient(t)
	if _, err := c.NewSession("alpha", "w", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewSession("beta", "w1", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	target, err := c.NewWindow("beta", "w2", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewWindow("beta", "w3", t.TempDir()); err != nil { // the current window is not the target
		t.Fatal(err)
	}
	connected := map[string]bool{} // ttys that look like they run ssh web
	procs := capture.Lister(func(tty string) ([]string, error) {
		if connected[tty] {
			return []string{"ssh web"}, nil
		}
		return []string{"-zsh"}, nil
	})
	cfg := Config{Client: c, SSH: twoHosts(), HostUse: hostuse.Finder{Client: c, Procs: procs}}
	pr, pw := io.Pipe()
	out := &syncBuf{}
	h := &hub{}
	p := tea.NewProgram(newModel(cfg, h), tea.WithInput(pr), tea.WithOutput(out), tea.WithAltScreen())
	h.send = p.Send
	go func() { _, _ = p.Run() }()
	t.Cleanup(func() { p.Kill(); _ = pw.Close() })

	p.Send(tea.WindowSizeMsg{Width: 120, Height: 30})
	waitFor(t, "client attached", func() bool {
		o, err := exec.Command("tmux", "-L", c.Socket, "list-clients", "-F", "#{client_session}").Output()
		return err == nil && strings.TrimSpace(string(o)) == "alpha"
	})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	waitFor(t, "hosts view", func() bool { return strings.Contains(out.String(), "web") })

	// No open pane: a new window in alpha runs ssh.
	p.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitFor(t, "new ssh window", func() bool {
		o, _ := exec.Command("tmux", "-L", c.Socket, "list-windows", "-t", "=alpha:", "-F", "#{window_name}").Output()
		b, _ := os.ReadFile(filepath.Join(bin, "args"))
		return strings.Contains(string(o), "web") && strings.TrimSpace(string(b)) == "fake ssh web"
	})

	// One open pane, in beta's second window: Enter goes there.
	refs, err := c.ListAllPanes()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		if r.Pane == target {
			connected[r.TTY] = true
		}
	}
	p.Send(tea.KeyMsg{Type: tea.KeyEsc}) // focus is in the terminal after the new window
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s"), Alt: true})
	p.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitFor(t, "client on beta's second window", func() bool {
		o, err := exec.Command("tmux", "-L", c.Socket, "list-clients", "-F", "#{client_session}|#{window_index}").Output()
		return err == nil && strings.TrimSpace(string(o)) == "beta|2"
	})
}

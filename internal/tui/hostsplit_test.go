package tui

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHostSplitNeedsAnOpenWorkspace(t *testing.T) {
	m := openHosts(t, twoHosts())
	m, cmd := press(m, "v")
	if cmd != nil || m.notice != "open a workspace first" {
		t.Errorf("notice %q, cmd %v", m.notice, cmd != nil)
	}
	m, cmd = press(m, "s")
	if cmd != nil || m.notice != "open a workspace first" {
		t.Errorf("s: notice %q, cmd %v", m.notice, cmd != nil)
	}
}

func TestHostSplitDoesNothingWithoutHosts(t *testing.T) {
	m := openHosts(t, &fakeHosts{})
	m, cmd := press(m, "v")
	if cmd != nil || m.notice != "" {
		t.Errorf("notice %q, cmd %v", m.notice, cmd != nil)
	}
}

func TestHostSplitFailureBecomesNotice(t *testing.T) {
	m := openHosts(t, twoHosts())
	next, _ := m.Update(hostSplitMsg{alias: "web", err: io.ErrClosedPipe})
	if got := next.(model).notice; got != io.ErrClosedPipe.Error() {
		t.Errorf("notice %q", got)
	}
}

// TestHostSplitRunsSSHInANewPane drives the whole program on a throwaway tmux
// server with a stand-in ssh: v puts the new pane beside the active one, s
// below it, and both run ssh to the selected host.
func TestHostSplitRunsSSHInANewPane(t *testing.T) {
	for _, tc := range []struct{ key, axis string }{{"v", "#{pane_left}"}, {"s", "#{pane_top}"}} {
		t.Run(tc.key, func(t *testing.T) {
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
			pr, pw := io.Pipe()
			out := &syncBuf{}
			h := &hub{}
			cfg := Config{Client: c, SSH: twoHosts()}
			p := tea.NewProgram(newModel(cfg, h), tea.WithInput(pr), tea.WithOutput(out), tea.WithAltScreen())
			h.send = p.Send
			go func() { _, _ = p.Run() }()
			t.Cleanup(func() { p.Kill(); _ = pw.Close() })

			p.Send(tea.WindowSizeMsg{Width: 120, Height: 30})
			waitFor(t, "client attached", func() bool {
				o, err := exec.Command("tmux", "-L", c.Socket, "list-clients").Output()
				return err == nil && len(o) > 0
			})
			p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
			waitFor(t, "hosts view", func() bool { return strings.Contains(out.String(), "web") })
			p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})

			waitFor(t, "second pane", func() bool {
				lines, err := c.ListPanes(tc.axis)
				return err == nil && len(lines) == 2
			})
			lines, _ := c.ListPanes(tc.axis)
			if lines[0] == lines[1] {
				t.Errorf("%s: panes share %s (%v), so they are not split that way", tc.key, tc.axis, lines)
			}
			waitFor(t, "ssh started with the alias", func() bool {
				b, _ := os.ReadFile(filepath.Join(bin, "args"))
				return strings.TrimSpace(string(b)) == "fake ssh web"
			})
		})
	}
}

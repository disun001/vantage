package tui

import (
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// driver steps a model by hand against a real tmux server, so tests can
// look at the model between messages instead of at a byte stream.
type driver struct {
	t *testing.T
	m model
}

func newDriver(t *testing.T, c Config) *driver {
	t.Helper()
	d := &driver{t: t, m: newModel(c, &hub{send: func(tea.Msg) {}})}
	t.Cleanup(func() {
		if d.m.term != nil {
			d.m.term.Close()
		}
	})
	d.send(tea.WindowSizeMsg{Width: 100, Height: 24})
	return d
}

func (d *driver) send(msg tea.Msg) tea.Cmd {
	next, cmd := d.m.Update(msg)
	d.m = next.(model)
	return cmd
}

// refresh runs one refresh round synchronously.
func (d *driver) refresh() { d.send(d.m.refresh()()) }

func tmuxCmd(t *testing.T, c Config, args ...string) {
	t.Helper()
	full := append([]string{"-L", c.Client.Socket}, args...)
	if out, err := exec.Command("tmux", full...).CombinedOutput(); err != nil {
		t.Fatalf("tmux %v: %v: %s", args, err, out)
	}
}

func TestTreeShowsWindowsAndHidesStatusBar(t *testing.T) {
	cfg := Config{Client: testClient(t)}
	c := cfg.Client
	if _, err := c.NewSession("alpha", "first", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewWindow("alpha", "uniqwin", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	tmuxCmd(t, cfg, "set", "-g", "status", "on")
	tmuxCmd(t, cfg, "set", "-g", "status-left", "ZZBAR")

	d := newDriver(t, cfg)
	waitFor(t, "status bar detected", func() bool { d.refresh(); return d.m.bar.lines == 1 })

	side := strings.Join(d.m.sidebar(), "\n")
	if !strings.Contains(side, "first") || !strings.Contains(side, "uniqwin") {
		t.Errorf("windows missing from the sidebar:\n%s", side)
	}
	if got := len(d.m.term.Lines(false)); got != d.m.h+1 {
		t.Errorf("embedded screen has %d lines, want window + bar = %d", got, d.m.h+1)
	}
	// The bar is on the edge row of the taller screen only once tmux has seen
	// the new client height, so wait for that. The edge depends on the user's
	// status-position (a top bar is on the first row from the start).
	waitFor(t, "status text drawn", func() bool {
		ls := d.m.term.Lines(false)
		if len(ls) == 0 {
			return false
		}
		edge := ls[len(ls)-1]
		if d.m.bar.top {
			edge = ls[0]
		}
		return strings.Contains(edge, "ZZBAR")
	})
	if v := d.m.View(); strings.Contains(v, "ZZBAR") {
		t.Errorf("status bar not cropped from the view:\n%s", v)
	}

	// Top bar: cropped from the top, mouse rows shifted past it.
	tmuxCmd(t, cfg, "set", "-g", "status-position", "top")
	waitFor(t, "top bar detected", func() bool { d.refresh(); return d.m.bar.top })
	if v := d.m.View(); strings.Contains(v, "ZZBAR") {
		t.Errorf("top status bar not cropped:\n%s", v)
	}

	// Turning it off gives the row back.
	tmuxCmd(t, cfg, "set", "-g", "status", "off")
	waitFor(t, "bar off", func() bool { d.refresh(); return d.m.bar.lines == 0 })
	if got := len(d.m.term.Lines(false)); got != d.m.h {
		t.Errorf("embedded screen has %d lines, want %d", got, d.m.h)
	}
}

func TestEnterOnWindowSelectsIt(t *testing.T) {
	cfg := Config{Client: testClient(t)}
	c := cfg.Client
	if _, err := c.NewSession("alpha", "first", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewWindow("alpha", "second", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	tmuxCmd(t, cfg, "select-window", "-t", "=alpha:^") // make the first one current

	d := newDriver(t, cfg)
	d.refresh()
	var second win
	for _, w := range d.m.rows[0].Wins {
		if w.Name == "second" {
			second = w
		}
	}
	if second.Name == "" || second.Active {
		t.Fatalf("setup: %+v", d.m.rows[0].Wins)
	}
	d.m.cur = key{"alpha", second.Index}
	msg := d.m.selectCmd()()
	if r, ok := msg.(readyMsg); !ok || r.err != nil {
		t.Fatalf("select: %#v", msg)
	}
	d.refresh()
	for _, w := range d.m.rows[0].Wins {
		if w.Active != (w.Name == "second") {
			t.Errorf("window %q active=%v after selecting second", w.Name, w.Active)
		}
	}
}

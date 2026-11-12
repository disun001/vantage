package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/manage"
	"github.com/disun001/vantage/internal/workspace"
)

// fakeManager records the calls the sidebar makes.
type fakeManager struct {
	calls []string
	err   error
}

func (f *fakeManager) Create(n string) error { f.calls = append(f.calls, "create "+n); return f.err }

func (f *fakeManager) Rename(a, b string) error {
	f.calls = append(f.calls, "rename "+a+" "+b)
	return f.err
}

func (f *fakeManager) Delete(n string) error { f.calls = append(f.calls, "delete "+n); return f.err }

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press sends keys one at a time and returns the model and the last command.
func press(m model, keys ...string) (model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		next, c := m.Update(keyMsg(k))
		m, cmd = next.(model), c
	}
	return m, cmd
}

func manageModel(f *fakeManager) model {
	m := treeModel()
	m.rows[0].Saved, m.rows[2].Saved = true, true // a and saved have files; b is live only
	m.cfg.Manage = f
	m.cfg.Client.Bin = "true" // a client that exits at once, so no tmux is needed
	m.hub = &hub{send: func(tea.Msg) {}}
	return m
}

func TestNewPrompt(t *testing.T) {
	f := &fakeManager{}
	m, _ := press(manageModel(f), "n", "w", "x", "backspace", "y")
	if m.prompt == nil || m.prompt.text != "wy" {
		t.Fatalf("prompt = %+v", m.prompt)
	}
	if !strings.Contains(m.sidebar()[m.h-1], "new: wy") {
		t.Errorf("footer = %q", m.sidebar()[m.h-1])
	}
	m, cmd := press(m, "enter")
	if m.prompt != nil || cmd == nil {
		t.Fatalf("enter must close the prompt and run: %+v", m.prompt)
	}
	msg := cmd()
	if len(f.calls) != 1 || f.calls[0] != "create wy" {
		t.Fatalf("calls = %v", f.calls)
	}
	next, _ := m.Update(msg)
	got := next.(model)
	if got.term != nil {
		defer got.term.Close()
	}
	if got.cur != (key{"wy", -1}) {
		t.Errorf("cursor = %+v, want it on the new session", got.cur)
	}
}

func TestPromptSwallowsKeysAndCancels(t *testing.T) {
	f := &fakeManager{}
	m, _ := press(manageModel(f), "n", "q", "j", "k")
	if m.prompt == nil || m.prompt.text != "qjk" {
		t.Fatalf("keys leaked out of the prompt: %+v", m.prompt)
	}
	if m.cur != (key{}) {
		t.Errorf("cursor moved to %+v while typing", m.cur)
	}
	m, cmd := press(m, "esc")
	if m.prompt != nil || cmd != nil || len(f.calls) != 0 {
		t.Fatalf("esc must cancel quietly: %v %v", m.prompt, f.calls)
	}
	m, cmd = press(m, "n", "enter") // empty name
	if m.prompt != nil || cmd != nil {
		t.Error("an empty name must do nothing")
	}
}

func TestRenamePromptStartsWithName(t *testing.T) {
	f := &fakeManager{}
	m := manageModel(f)
	m.cur = key{"a", 2} // on a window line: renames its session
	m, _ = press(m, "R")
	if m.prompt == nil || m.prompt.text != "a" || m.prompt.target != "a" {
		t.Fatalf("prompt = %+v", m.prompt)
	}
	m, cmd := press(m, "enter") // unchanged name: nothing to do
	if cmd != nil {
		t.Fatal("renaming to the same name must not run")
	}
	m, cmd = press(m, "R", "backspace", "z", "enter")
	cmd()
	if len(f.calls) != 1 || f.calls[0] != "rename a z" {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestRenameMovesCursorAndOpenState(t *testing.T) {
	m := manageModel(&fakeManager{})
	m.cur = key{"a", 2}
	m.open = map[string]bool{"a": true, "b": false}
	next, _ := m.Update(manageMsg{op: promptRename, from: "a", to: "z"})
	got := next.(model)
	if got.cur != (key{"z", 2}) || got.current != "z" || !got.open["z"] || got.open["b"] {
		t.Errorf("cur=%+v current=%q open=%v", got.cur, got.current, got.open)
	}
	if _, stale := got.open["a"]; stale {
		t.Error("old name left in the open map")
	}
}

func TestDeletePrompt(t *testing.T) {
	f := &fakeManager{}
	m := manageModel(f)
	m.cur = key{"b", -1} // live, not saved: deletable too
	m, _ = press(m, "d")
	if m.prompt == nil || m.prompt.target != "b" {
		t.Fatalf("prompt = %+v", m.prompt)
	}
	m, _ = press(m, "esc")
	m.cur = key{"saved", -1}
	m, _ = press(m, "d")
	if m.prompt == nil || !strings.Contains(m.sidebar()[m.h-1], "delete saved? y/n") {
		t.Fatalf("no confirm: %+v", m.prompt)
	}
	m, cmd := press(m, "x") // anything but y cancels
	if m.prompt != nil || cmd != nil || len(f.calls) != 0 {
		t.Fatal("a key other than y must cancel")
	}
	m, _ = press(m, "d")
	_, cmd = press(m, "y")
	cmd()
	if len(f.calls) != 1 || f.calls[0] != "delete saved" {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestDeleteParksCursorOnNeighbour(t *testing.T) {
	m := manageModel(&fakeManager{})
	m.cur = key{"saved", -1} // last row, not running
	if got := m.afterDelete("saved").cur; got != (key{"b", -1}) {
		t.Errorf("cursor = %+v, want the line above", got)
	}
	m.cur = key{"a", 2} // a window line of a live session: its whole row goes
	if got := m.afterDelete("a"); got.cur != (key{"b", -1}) || got.current != "" {
		t.Errorf("cursor = %+v current = %q, want b and no current", got.cur, got.current)
	}
}

func TestManageErrorShowsNotice(t *testing.T) {
	f := &fakeManager{err: errors.New("workspace \"x\" already exists")}
	m, cmd := press(manageModel(f), "n", "x", "enter")
	next, _ := m.Update(cmd())
	if got := next.(model).notice; !strings.Contains(got, "already exists") {
		t.Errorf("notice = %q", got)
	}
}

func TestKeysDoNothingWithoutManager(t *testing.T) {
	m := treeModel()
	m, _ = press(m, "n", "R", "d")
	if m.prompt != nil {
		t.Error("prompt opened with no manager configured")
	}
}

// TestManageEndToEnd drives n, R and d against a real tmux server and a
// real workspaces directory.
func TestManageEndToEnd(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	c := testClient(t)
	cfg := Config{Client: c, Saved: workspace.List, Manage: manage.Manager{Client: c}}
	d := newDriver(t, cfg)
	run := func(keys ...string) {
		t.Helper()
		var cmd tea.Cmd
		for _, k := range keys {
			cmd = d.send(keyMsg(k))
		}
		if cmd != nil {
			d.send(cmd())
		}
		d.refresh()
	}
	has := func(name string) (live, saved bool) {
		for _, r := range d.m.rows {
			if r.Name == name {
				return r.Live, r.Saved
			}
		}
		return false, false
	}

	run("n", "p", "r", "o", "j", "enter")
	if live, saved := has("proj"); !live || !saved {
		t.Fatalf("after new: live=%v saved=%v notice=%q", live, saved, d.m.notice)
	}
	if d.m.cur != (key{"proj", -1}) || d.m.current != "proj" || d.m.term == nil {
		t.Errorf("new session not opened: cur=%+v current=%q", d.m.cur, d.m.current)
	}

	d.m.focus = focusSidebar
	run("R", "backspace", "backspace", "backspace", "backspace", "n", "e", "w", "enter")
	if live, saved := has("new"); !live || !saved {
		t.Fatalf("after rename: live=%v saved=%v notice=%q", live, saved, d.m.notice)
	}
	if live, saved := has("proj"); live || saved {
		t.Error("old name still listed")
	}
	if d.m.cur.Session != "new" {
		t.Errorf("cursor lost in rename: %+v", d.m.cur)
	}

	run("d", "y")
	if live, saved := has("new"); live || saved {
		t.Fatalf("after delete: live=%v saved=%v notice=%q", live, saved, d.m.notice)
	}
	if ok, _ := workspace.Exists("new"); ok || c.HasSession("new") {
		t.Error("file or session still there")
	}
	if _, err := os.Stat(os.Getenv("VANTAGE_CONFIG_DIR")); err != nil {
		t.Error(err)
	}
}

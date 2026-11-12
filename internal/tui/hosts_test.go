package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/disun001/vantage/internal/hostuse"
	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/tmux"
)

// fakeHosts is an in-memory HostStore. Hosts named in shared cannot be edited.
type fakeHosts struct {
	hosts  []sshconfig.Host
	shared map[string]bool
	calls  []string
	err    error
}

func (f *fakeHosts) List() ([]sshconfig.Host, error) { return f.hosts, f.err }

func (f *fakeHosts) Own(alias string) (sshconfig.Host, error) {
	if f.shared[alias] {
		return sshconfig.Host{}, sshconfig.ErrShared
	}
	for _, h := range f.hosts {
		if h.Alias == alias {
			return h, nil
		}
	}
	return sshconfig.Host{}, sshconfig.ErrNotFound
}

func (f *fakeHosts) Add(h sshconfig.Host) error {
	f.calls = append(f.calls, "add "+h.Alias)
	if f.err == nil {
		f.hosts = append(f.hosts, h)
	}
	return f.err
}

func (f *fakeHosts) Edit(h sshconfig.Host) error {
	f.calls = append(f.calls, "edit "+h.Alias)
	for i := range f.hosts {
		if f.hosts[i].Alias == h.Alias && f.err == nil {
			f.hosts[i] = h
		}
	}
	return f.err
}

func (f *fakeHosts) Delete(alias string) error {
	f.calls = append(f.calls, "delete "+alias)
	if f.err != nil {
		return f.err
	}
	for i, h := range f.hosts {
		if h.Alias == alias {
			f.hosts = append(f.hosts[:i:i], f.hosts[i+1:]...)
			break
		}
	}
	return nil
}

func hostModel(f *fakeHosts) model {
	m := manageModel(&fakeManager{})
	if f != nil { // a nil pointer in the interface would not be nil
		m.cfg.SSH = f
	}
	return m
}

// settle runs a command and feeds its message back, like the Bubble Tea loop.
func settle(m model, cmd tea.Cmd) model {
	if cmd == nil {
		return m
	}
	next, _ := m.Update(cmd())
	return next.(model)
}

// settleAll keeps feeding commands back until none is left, so a save is
// followed by the reload it asks for.
func settleAll(m model, cmd tea.Cmd) model {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if b, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, b...)
			continue
		}
		if _, ok := msg.(rowsMsg); ok {
			continue // the stub tmux has no sessions to list; not what these tests are about
		}
		next, more := m.Update(msg)
		m = next.(model)
		queue = append(queue, more)
	}
	return m
}

// typed presses each rune of s as its own key.
func typed(m model, s string) model {
	for _, r := range s {
		m, _ = press(m, string(r))
	}
	return m
}

func openHosts(t *testing.T, f *fakeHosts) model {
	t.Helper()
	m, cmd := press(hostModel(f), "H")
	if m.view != viewHosts {
		t.Fatal("H did not open the hosts view")
	}
	return settle(m, cmd)
}

var twoHosts = func() *fakeHosts {
	return &fakeHosts{
		hosts:  []sshconfig.Host{{Alias: "web", HostName: "192.0.2.1", User: "deploy"}, {Alias: "db", HostName: "192.0.2.2"}, {Alias: "grp"}},
		shared: map[string]bool{"grp": true},
	}
}

func TestHostsViewListsAndMoves(t *testing.T) {
	m := openHosts(t, twoHosts())
	if len(m.hosts) != 3 || m.hostCur != "web" {
		t.Fatalf("hosts=%v cur=%q", m.hosts, m.hostCur)
	}
	side := strings.Join(m.sidebar(), "\n")
	for _, want := range []string{"ssh hosts", "web", "deploy@192.0.2.1", "db"} {
		if !strings.Contains(side, want) {
			t.Errorf("sidebar lacks %q:\n%s", want, side)
		}
	}
	m, _ = press(m, "j", "j", "j") // clamps at the last host
	if m.hostCur != "grp" {
		t.Errorf("cur = %q", m.hostCur)
	}
	m, _ = press(m, "g")
	if m.hostCur != "web" {
		t.Errorf("g: cur = %q", m.hostCur)
	}
	m, cmd := press(m, "H")
	if m.view != viewSessions || cmd != nil {
		t.Errorf("H must go back to sessions: view=%v", m.view)
	}
}

func TestHostsViewOffWithoutStore(t *testing.T) {
	m := manageModel(&fakeManager{})
	if m, cmd := press(m, "H"); m.view != viewSessions || cmd != nil {
		t.Error("H must do nothing without a store")
	}
}

func TestAddHostForm(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "a")
	if m.form == nil || m.form.edit || m.form.focus != 0 {
		t.Fatalf("form = %+v", m.form)
	}
	m = typed(m, "new")
	m, _ = press(m, "tab")
	m = typed(m, "192.0.2.9")
	m, _ = press(m, "tab", "tab") // skip user
	m = typed(m, "2200")
	m, _ = press(m, "tab")
	m = typed(m, "web")
	m, cmd := press(m, "enter")
	if m.form != nil || cmd == nil {
		t.Fatalf("Enter must save: form=%+v", m.form)
	}
	m = settleAll(m, cmd)
	want := sshconfig.Host{Alias: "new", HostName: "192.0.2.9", Port: "2200", ProxyJump: "web"}
	if got := f.hosts[len(f.hosts)-1]; got != want || !reflect.DeepEqual(f.calls, []string{"add new"}) {
		t.Errorf("saved %+v, calls %v", got, f.calls)
	}
	if m.hostCur != "new" {
		t.Errorf("cursor should land on the new host, got %q", m.hostCur)
	}
}

func TestFormTabWrapsAndShiftTabGoesBack(t *testing.T) {
	m := openHosts(t, twoHosts())
	m, _ = press(m, "a", "shift+tab")
	if m.form.focus != 4 {
		t.Errorf("shift+tab from the top: focus %d", m.form.focus)
	}
	m, _ = press(m, "tab")
	if m.form.focus != 0 {
		t.Errorf("tab from the bottom: focus %d", m.form.focus)
	}
	m, _ = press(m, "esc", "e", "shift+tab") // editing never lands on the alias
	if m.form.focus != 4 {
		t.Fatalf("edit shift+tab: focus %d", m.form.focus)
	}
	m, _ = press(m, "tab")
	if m.form.focus != 1 {
		t.Errorf("edit tab wraps to %d, want 1", m.form.focus)
	}
}

func TestFormStopsOnInvalidField(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "a")
	m = typed(m, "n")
	m, _ = press(m, "tab")
	m = typed(m, "bad host") // the space is dropped, not typed
	if m.form.vals[1] != "badhost" {
		t.Fatalf("text = %q", m.form.vals[1])
	}
	m, _ = press(m, "ctrl+u")
	m = typed(m, "x;y")
	m, _ = press(m, "tab", "tab", "tab") // wander off to another field
	m, cmd := press(m, "enter")
	if m.form == nil || m.form.focus != 1 || m.form.err == "" || cmd != nil {
		t.Fatalf("an invalid hostname must keep the form open on it: %+v", m.form)
	}
	if !strings.Contains(strings.Join(m.formBox(), "\n"), "hostname") || len(f.calls) != 0 {
		t.Errorf("calls = %v", f.calls)
	}
	m, _ = press(m, "esc", "a", "enter") // alias is required
	if m.form == nil || m.form.err != "host is required" {
		t.Errorf("empty alias: %+v", m.form)
	}
}

func TestFormCancel(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "a", "x", "esc")
	if m.form != nil || len(f.calls) != 0 {
		t.Errorf("esc must drop the form: %+v calls=%v", m.form, f.calls)
	}
}

func TestEditHostForm(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "e")
	if m.form == nil || !m.form.edit || m.form.focus != 1 || m.form.vals[1] != "192.0.2.1" || m.form.vals[2] != "deploy" {
		t.Fatalf("form = %+v", m.form)
	}
	m, _ = press(m, "backspace", "9") // 192.0.2.9
	m, cmd := press(m, "enter")
	if m.form != nil {
		t.Fatalf("form still open: %+v", m.form)
	}
	m = settleAll(m, cmd)
	want := sshconfig.Host{Alias: "web", HostName: "192.0.2.9", User: "deploy"}
	if f.hosts[0] != want || !reflect.DeepEqual(f.calls, []string{"edit web"}) {
		t.Errorf("hosts[0] = %+v, calls %v", f.hosts[0], f.calls)
	}
}

func TestFormPopsUpOverTheScreen(t *testing.T) {
	m := openHosts(t, twoHosts())
	m.w, m.h = 80, 24
	plain := strings.Split(m.View(), "\n")
	m, _ = press(m, "a", "x")
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("height %d", len(lines))
	}
	all := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"add host", "hostname", "user", "port", "jump", "tab next", "x"} {
		if !strings.Contains(all, want) {
			t.Errorf("screen lacks %q", want)
		}
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("line %d is %d cells wide", i, w)
		}
	}
	if plain[0] != lines[0] {
		t.Error("the top line should be untouched by a centred box")
	}
}

func TestEditSharedHostSaysWhy(t *testing.T) {
	m := openHosts(t, twoHosts())
	m, _ = press(m, "G", "e")
	if m.form != nil || !strings.Contains(m.notice, "grp") || !strings.Contains(m.notice, "by hand") {
		t.Errorf("form=%+v notice=%q", m.form, m.notice)
	}
}

func TestSaveErrorBecomesNotice(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "a", "n")
	f.err = errors.New("disk full")
	m, cmd := press(m, "enter")
	m = settle(m, cmd)
	if !strings.Contains(m.notice, "disk full") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestMouseIgnoredWhileFormOpen(t *testing.T) {
	m := openHosts(t, twoHosts())
	m, _ = press(m, "a")
	next, _ := m.Update(tea.MouseMsg{X: 2, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := next.(model); got.hostCur != "web" {
		t.Errorf("click moved the cursor under a form: %q", got.hostCur)
	}
}

func TestMouseSelectsHost(t *testing.T) {
	m := openHosts(t, twoHosts())
	next, _ := m.Update(tea.MouseMsg{X: 2, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := next.(model); got.hostCur != "db" {
		t.Errorf("cur = %q", got.hostCur)
	}
}

// The real store: keys in, a fixture file out, never the user's ~/.ssh.
func TestHostsEndToEndOnTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	orig := "Host web\n  HostName 192.0.2.1\n  IdentityFile k\n\nHost *\n  ServerAliveInterval 30\n"
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	m := hostModel(nil)
	m.cfg.SSH = sshconfig.File{Path: path}
	m, cmd := press(m, "H")
	m = settleAll(m, cmd)
	m, _ = press(m, "e", "backspace", "5") // 192.0.2.5
	m, cmd = press(m, "enter")
	m = settleAll(m, cmd)
	m, _ = press(m, "a")
	m = typed(m, "db")
	m, cmd = press(m, "enter")
	m = settleAll(m, cmd)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "Host web\n  HostName 192.0.2.5\n  IdentityFile k\n\nHost db\n\nHost *\n  ServerAliveInterval 30\n"
	if string(got) != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
	if m.notice != "" || m.hostCur != "db" || len(m.hosts) != 2 {
		t.Errorf("notice=%q cur=%q hosts=%v", m.notice, m.hostCur, m.hosts)
	}
}

func TestDeleteHost(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "j", "d") // db
	if m.hostDel != "db" || !strings.Contains(m.sidebar()[m.h-1], "delete db? y/n") {
		t.Fatalf("hostDel=%q footer=%q", m.hostDel, m.sidebar()[m.h-1])
	}
	m, cmd := press(m, "n")
	if m.hostDel != "" || cmd != nil || len(f.calls) != 0 {
		t.Fatalf("any other key must cancel: %q calls=%v", m.hostDel, f.calls)
	}
	m, cmd = press(m, "d", "y")
	if cmd == nil {
		t.Fatal("y must delete")
	}
	m = settleAll(m, cmd)
	if !reflect.DeepEqual(f.calls, []string{"delete db"}) || len(m.hosts) != 2 {
		t.Errorf("calls=%v hosts=%v", f.calls, m.hosts)
	}
	if m.hostCur != "grp" { // the host below the deleted one
		t.Errorf("cursor = %q", m.hostCur)
	}
	m, _ = press(m, "d", "y") // grp is last: its neighbour is the one above
	m = settleAll(m, func() tea.Msg { return hostSavedMsg{alias: "grp", deleted: true} })
	if m.hostCur != "web" {
		t.Errorf("cursor = %q", m.hostCur)
	}
}

func TestDeleteHostErrorAndMouse(t *testing.T) {
	f := twoHosts()
	f.shared["db"] = true
	m := openHosts(t, f)
	m, _ = press(m, "d")
	next, _ := m.Update(tea.MouseMsg{X: 2, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := next.(model); got.hostCur != "web" {
		t.Errorf("click moved the cursor during the question: %q", got.hostCur)
	}
	f.err = errors.New("read-only file")
	m, cmd := press(m, "y")
	m = settleAll(m, cmd)
	if !strings.Contains(m.notice, "read-only file") || len(m.hosts) != 3 {
		t.Errorf("notice=%q hosts=%v", m.notice, m.hosts)
	}
}

func TestDeleteNothingWhenEmpty(t *testing.T) {
	m := openHosts(t, &fakeHosts{})
	if m, _ = press(m, "d"); m.hostDel != "" {
		t.Error("asked to delete from an empty list")
	}
}

// fakeUse reports a fixed usage and records what gets closed.
type fakeUse struct {
	use       hostuse.Usage
	findErr   error
	closeErr  error
	stripErr  error
	closed    []string
	stripped  []string
	findCalls int
}

func (f *fakeUse) Strip(alias string) error {
	f.stripped = append(f.stripped, alias)
	return f.stripErr
}

func (f *fakeUse) Find(string) (hostuse.Usage, error) { f.findCalls++; return f.use, f.findErr }

func (f *fakeUse) Close(u hostuse.Usage) error {
	for _, p := range u.Panes {
		f.closed = append(f.closed, p.Pane)
	}
	return f.closeErr
}

func usedHost(t *testing.T, u *fakeUse) (model, *fakeHosts) {
	t.Helper()
	f := twoHosts()
	m := openHosts(t, f)
	m.cfg.HostUse = u
	return m, f
}

func TestDeleteWarnsThenClosesPanes(t *testing.T) {
	u := &fakeUse{use: hostuse.Usage{
		Workspaces: []string{"proj", "ops"},
		Panes:      []tmux.PaneRef{{Pane: "%3"}, {Pane: "%7"}},
	}}
	m, f := usedHost(t, u)
	m, cmd := press(m, "d")
	if m.hostDel != "" || cmd == nil {
		t.Fatal("the question comes after the lookup")
	}
	m = settle(m, cmd)
	if m.hostDel != "web" {
		t.Fatalf("hostDel = %q", m.hostDel)
	}
	for _, want := range []string{"proj, ops", "ssh panes will be removed", "2 open pane"} {
		if !strings.Contains(m.notice, want) {
			t.Errorf("warning %q lacks %q", m.notice, want)
		}
	}
	if len(f.calls) != 0 || len(u.closed) != 0 {
		t.Fatal("nothing may happen before y")
	}
	m, cmd = press(m, "y")
	if m.notice != "" {
		t.Errorf("warning must clear: %q", m.notice)
	}
	m = settleAll(m, cmd)
	if !reflect.DeepEqual(f.calls, []string{"delete web"}) || !reflect.DeepEqual(u.closed, []string{"%3", "%7"}) {
		t.Errorf("calls=%v closed=%v", f.calls, u.closed)
	}
	if !reflect.DeepEqual(u.stripped, []string{"web"}) {
		t.Errorf("workspaces stripped for %v, want web", u.stripped)
	}
	if u.findCalls != 2 {
		t.Errorf("usage must be looked up again at confirm time, got %d lookups", u.findCalls)
	}
}

func TestDeleteWarningCancelClosesNothing(t *testing.T) {
	u := &fakeUse{use: hostuse.Usage{Panes: []tmux.PaneRef{{Pane: "%1"}}}}
	m, f := usedHost(t, u)
	m, cmd := press(m, "d")
	m = settle(m, cmd)
	m, cmd = press(m, "n")
	if cmd != nil || m.notice != "" || m.hostDel != "" || len(f.calls) != 0 || len(u.closed) != 0 || len(u.stripped) != 0 {
		t.Errorf("cancel did something: notice=%q calls=%v closed=%v stripped=%v", m.notice, f.calls, u.closed, u.stripped)
	}
}

func TestDeleteUnusedHostJustAsks(t *testing.T) {
	m, _ := usedHost(t, &fakeUse{})
	m, cmd := press(m, "d")
	m = settle(m, cmd)
	if m.hostDel != "web" || m.notice != "" {
		t.Errorf("hostDel=%q notice=%q", m.hostDel, m.notice)
	}
}

func TestDeleteKeepsPanesWhenHostDeleteFails(t *testing.T) {
	u := &fakeUse{use: hostuse.Usage{Panes: []tmux.PaneRef{{Pane: "%1"}}}}
	m, f := usedHost(t, u)
	m, cmd := press(m, "d")
	m = settle(m, cmd)
	f.err = errors.New("read-only file")
	m, cmd = press(m, "y")
	m = settleAll(m, cmd)
	if len(u.closed) != 0 || len(u.stripped) != 0 || !strings.Contains(m.notice, "read-only file") {
		t.Errorf("closed=%v stripped=%v notice=%q", u.closed, u.stripped, m.notice)
	}
}

func TestDeleteReportsIncompleteCheckAndCloseFailure(t *testing.T) {
	u := &fakeUse{findErr: errors.New("bad.toml: broken"), closeErr: errors.New("tmux gone"), stripErr: errors.New("solo: no windows")}
	m, _ := usedHost(t, u)
	m, cmd := press(m, "d")
	m = settle(m, cmd)
	if m.hostDel != "web" || !strings.Contains(m.notice, "could not check everything: bad.toml: broken") {
		t.Fatalf("hostDel=%q notice=%q", m.hostDel, m.notice)
	}
	m, cmd = press(m, "y")
	m = settleAll(m, cmd)
	for _, want := range []string{"closing its panes failed", "workspaces not fully updated: solo: no windows"} {
		if !strings.Contains(m.notice, want) {
			t.Errorf("notice %q lacks %q", m.notice, want)
		}
	}
}

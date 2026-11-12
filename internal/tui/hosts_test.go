package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/sshconfig"
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
	for cmd != nil {
		next, c := m.Update(cmd())
		m, cmd = next.(model), c
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
	if m.form == nil || !strings.Contains(m.sidebar()[m.h-1], "host: ") {
		t.Fatalf("form = %+v", m.form)
	}
	m = typed(m, "new")
	m, _ = press(m, "enter")
	m = typed(m, "192.0.2.9")
	m, _ = press(m, "enter", "enter") // no user
	m = typed(m, "2200")
	if !strings.Contains(m.sidebar()[m.h-1], "port: 2200") {
		t.Errorf("footer = %q", m.sidebar()[m.h-1])
	}
	m, _ = press(m, "enter")
	m = typed(m, "web")
	m, cmd := press(m, "enter")
	if m.form != nil || cmd == nil {
		t.Fatalf("last Enter must save: form=%+v", m.form)
	}
	m = settle(m, cmd)
	want := sshconfig.Host{Alias: "new", HostName: "192.0.2.9", Port: "2200", ProxyJump: "web"}
	if got := f.hosts[len(f.hosts)-1]; got != want || !reflect.DeepEqual(f.calls, []string{"add new"}) {
		t.Errorf("saved %+v, calls %v", got, f.calls)
	}
	if m.hostCur != "new" {
		t.Errorf("cursor should land on the new host, got %q", m.hostCur)
	}
}

func TestFormKeepsStepOnInvalidInput(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "a")
	m = typed(m, "n")
	m, _ = press(m, "enter")
	m = typed(m, "bad host") // the space is dropped, not typed
	if m.form.text != "badhost" {
		t.Fatalf("text = %q", m.form.text)
	}
	m, _ = press(m, "ctrl+u")
	m = typed(m, "x;y")
	m, _ = press(m, "enter")
	if m.form == nil || m.form.step != 1 || m.notice == "" {
		t.Fatalf("an invalid hostname must stay on its step: %+v notice=%q", m.form, m.notice)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestFormCancel(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "a", "enter") // empty alias
	if m.form != nil {
		t.Error("Enter on an empty alias must cancel")
	}
	m, _ = press(m, "a", "x", "enter", "esc")
	if m.form != nil || len(f.calls) != 0 {
		t.Errorf("esc must drop the form: %+v calls=%v", m.form, f.calls)
	}
}

func TestEditHostForm(t *testing.T) {
	f := twoHosts()
	m := openHosts(t, f)
	m, _ = press(m, "e")
	if m.form == nil || !m.form.edit || m.form.text != "192.0.2.1" || m.form.step != 1 {
		t.Fatalf("form = %+v", m.form)
	}
	m, _ = press(m, "backspace", "9") // 192.0.2.9
	m, _ = press(m, "enter")
	if m.form.text != "deploy" {
		t.Fatalf("user step should be prefilled, got %q", m.form.text)
	}
	m, cmd := press(m, "enter", "enter", "enter") // keep user, port and jump; the last Enter saves
	if m.form != nil {
		t.Fatalf("form still open: %+v", m.form)
	}
	m = settle(m, cmd)
	want := sshconfig.Host{Alias: "web", HostName: "192.0.2.9", User: "deploy"}
	if f.hosts[0] != want || !reflect.DeepEqual(f.calls, []string{"edit web"}) {
		t.Errorf("hosts[0] = %+v, calls %v", f.hosts[0], f.calls)
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
	m, _ = press(m, "a", "n", "enter", "enter", "enter", "enter")
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
	m, cmd = press(m, "enter", "enter", "enter", "enter")
	m = settleAll(m, cmd)
	m, _ = press(m, "a")
	m = typed(m, "db")
	m, cmd = press(m, "enter", "enter", "enter", "enter", "enter")
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

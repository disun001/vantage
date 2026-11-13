package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func ansiText(s string) string { return ansi.Strip(s) }

// fakeProjects is an in-memory ProjectStore.
type fakeProjects struct {
	infos map[string]ProjectInfo
	calls []string
	err   error // fails NewTask and Create
}

func (f *fakeProjects) List() ([]string, error) {
	var names []string
	for _, n := range []string{"api-stack", "solo"} {
		if _, ok := f.infos[n]; ok {
			names = append(names, n)
		}
	}
	return names, nil
}

func (f *fakeProjects) Detail(name string) (ProjectInfo, error) { return f.infos[name], nil }

func (f *fakeProjects) Create(d ProjectDef) error {
	f.calls = append(f.calls, fmt.Sprintf("create %+v", d))
	return f.err
}

func (f *fakeProjects) NewTask(p, task string, picks []TaskPick) error {
	f.calls = append(f.calls, fmt.Sprintf("newtask %s %s %v", p, task, picks))
	return f.err
}

func (f *fakeProjects) Workspace(p, task string) (string, error) {
	f.calls = append(f.calls, "workspace "+p+" "+task)
	return p + "-" + task, nil
}

func stackInfos() map[string]ProjectInfo {
	return map[string]ProjectInfo{
		"api-stack": {
			Name: "api-stack",
			Repos: []RepoInfo{
				{Name: "api", Cloned: true, Branch: "main", Sync: "up to date"},
				{Name: "web", Cloned: true, Branch: "dev", Sync: "+1 -0", Dirty: true, Warn: "on dev, default is main"},
				{Name: "docs"},
			},
			Tasks: []TaskInfo{{Name: "fix-login", Repos: []string{"api", "web"}}, {Name: "other", Repos: []string{"web"}}},
		},
		"solo": {Name: "solo", Flat: true, Repos: []RepoInfo{{Name: "solo", Cloned: true, Branch: "main", Sync: "up to date"}}},
	}
}

func projectModel(f *fakeProjects, restored *[]string) model {
	m := manageModel(&fakeManager{})
	m.cfg.Projects = f
	m.cfg.Restore = func(name string) error { *restored = append(*restored, name); return nil }
	return m
}

func openProjects(t *testing.T, f *fakeProjects, restored *[]string) model {
	t.Helper()
	m, cmd := press(projectModel(f, restored), "P")
	if m.view != viewProjects {
		t.Fatal("P did not open the projects view")
	}
	return settleAll(m, cmd)
}

func TestProjectsViewListsAndShowsDetails(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	if len(m.projects) != 2 || m.projCur != "api-stack" || m.projInfo == nil {
		t.Fatalf("projects=%v cur=%q info=%v", m.projects, m.projCur, m.projInfo)
	}
	side := ansiText(strings.Join(m.sidebar(), "\n"))
	for _, want := range []string{"projects", "api-stack", "solo"} {
		if !strings.Contains(side, want) {
			t.Errorf("sidebar lacks %q:\n%s", want, side)
		}
	}
	m.w, m.h = 100, 30
	right := ansiText(strings.Join(m.right(), "\n"))
	for _, want := range []string{"api-stack", "up to date", "dirty", "on dev, default is main", "not cloned", "fix-login", "api, web"} {
		if !strings.Contains(right, want) {
			t.Errorf("details lack %q:\n%s", want, right)
		}
	}
}

func TestProjectsViewOffWithoutStore(t *testing.T) {
	m, _ := press(manageModel(&fakeManager{}), "P")
	if m.view != viewSessions {
		t.Error("P must do nothing without a store")
	}
}

func TestProjectsMoveLoadsDetailsAndDropsStaleAnswers(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	m, cmd := press(m, "j")
	if m.projCur != "solo" || m.projInfo != nil {
		t.Fatalf("cur=%q info=%v (details must clear while loading)", m.projCur, m.projInfo)
	}
	stale, _ := m.Update(projDetailMsg{name: "api-stack", info: ProjectInfo{Name: "api-stack"}})
	if stale.(model).projInfo != nil {
		t.Error("an answer for a project the cursor left must be dropped")
	}
	m = settle(m, cmd)
	if m.projInfo == nil || m.projInfo.Name != "solo" || !m.projInfo.Flat {
		t.Errorf("info = %+v", m.projInfo)
	}
	if m, _ = press(m, "j"); m.projCur != "solo" {
		t.Error("cursor must clamp at the last project")
	}
}

func TestTaskListFocus(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	m, _ = press(m, "enter")
	if !m.projFocus {
		t.Fatal("enter must step into the task list")
	}
	m, _ = press(m, "j", "j")
	if m.taskCur != 1 {
		t.Errorf("taskCur = %d, want clamped at 1", m.taskCur)
	}
	if m, _ = press(m, "esc"); m.projFocus || m.view != viewProjects {
		t.Error("esc steps out of the list, not out of the view")
	}
	// solo has no tasks: nothing to step into.
	m, cmd := press(m, "j")
	m = settle(m, cmd)
	if m, _ = press(m, "enter"); m.projFocus {
		t.Error("no tasks, no focus")
	}
	if m, _ = press(m, "esc"); m.view != viewSessions {
		t.Error("esc from the list returns to workspaces")
	}
}

func TestOpenTaskReturnsToWorkspaceAndRestores(t *testing.T) {
	f, restored := &fakeProjects{infos: stackInfos()}, new([]string)
	m := openProjects(t, f, restored)
	m, _ = press(m, "enter", "j")
	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter on a task must act")
	}
	next, _ := m.Update(cmd())
	got := next.(model)
	if got.term != nil {
		defer got.term.Close()
	}
	if got.view != viewSessions || got.projFocus {
		t.Errorf("view=%v focus=%v, want back on workspaces", got.view, got.projFocus)
	}
	if got.cur != (key{"api-stack-other", -1}) {
		t.Errorf("cursor = %+v, want the task's workspace", got.cur)
	}
	if len(*restored) != 1 || (*restored)[0] != "api-stack-other" {
		t.Errorf("restored = %v", *restored)
	}
	if got.current != "api-stack-other" {
		t.Errorf("shown session = %q", got.current)
	}
	if strings.Join(f.calls, ";") != "workspace api-stack other" {
		t.Errorf("an existing task must not be created again: %v", f.calls)
	}
}

func TestNewTaskForm(t *testing.T) {
	f, restored := &fakeProjects{infos: stackInfos()}, new([]string)
	m := openProjects(t, f, restored)
	m, _ = press(m, "n")
	if m.tform == nil {
		t.Fatal("n did not open the task form")
	}
	box := ansiText(popupText(m))
	for _, want := range []string{"new task in api-stack", "[x] api", "[x] web", "default branch"} {
		if !strings.Contains(box, want) {
			t.Errorf("form lacks %q:\n%s", want, box)
		}
	}
	m = typed(m, "t1")
	m, _ = press(m, "tab", "tab") // web
	m = typed(m, "dev")
	m, _ = press(m, "tab", "space") // docs: leave it out
	if r := m.tform.repos; !r[0].include || !r[1].include || r[2].include {
		t.Fatalf("space must toggle the focused repo only: %+v", r)
	}
	m, cmd := press(m, "enter")
	if m.tform != nil || cmd == nil {
		t.Fatalf("enter must close the form and run: %+v", m.tform)
	}
	next, _ := m.Update(cmd())
	got := next.(model)
	if got.term != nil {
		defer got.term.Close()
	}
	want := []string{"newtask api-stack t1 [{api } {web dev}]", "workspace api-stack t1"}
	if strings.Join(f.calls, ";") != strings.Join(want, ";") {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
	if got.view != viewSessions || got.cur != (key{"api-stack-t1", -1}) {
		t.Errorf("view=%v cur=%+v", got.view, got.cur)
	}
}

func TestNewTaskFormRefusals(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	m, cmd := press(m, "n", "enter")
	if m.tform == nil || cmd != nil || !strings.Contains(m.tform.err, "name") {
		t.Fatalf("empty name must stay open with a message: %+v", m.tform)
	}
	m = typed(m, "t")
	m, _ = press(m, "tab", "space", "tab", "space", "tab", "space", "enter")
	if m.tform == nil || !strings.Contains(m.tform.err, "at least one repo") {
		t.Fatalf("no repos must stay open with a message: %+v", m.tform)
	}
	if m, _ = press(m, "esc"); m.tform != nil {
		t.Error("esc closes the form")
	}
}

func TestNewTaskFailureStaysOnDetails(t *testing.T) {
	f := &fakeProjects{infos: stackInfos(), err: errors.New("branch \"t\" already exists")}
	m := openProjects(t, f, new([]string))
	m, _ = press(m, "n")
	m = typed(m, "t")
	m, cmd := press(m, "enter")
	m = settleAll(m, cmd)
	if m.view != viewProjects || !strings.Contains(m.notice, "already exists") {
		t.Errorf("view=%v notice=%q", m.view, m.notice)
	}
}

func TestAddProjectForm(t *testing.T) {
	f := &fakeProjects{infos: stackInfos()}
	m := openProjects(t, f, new([]string))
	m, _ = press(m, "a")
	if m.pform == nil {
		t.Fatal("a did not open the project form")
	}
	m = typed(m, "newp")
	m, _ = press(m, "tab", "tab") // skip flat → repo 1 url
	m = typed(m, "git@example.invalid:o/a.git")
	m, _ = press(m, "tab", "tab") // dir blank → copy
	m = typed(m, ".env,cfg/*.json")
	m, _ = press(m, "ctrl+a") // a second repo, focus on its url
	m = typed(m, "https://example.invalid/o/b")
	m, _ = press(m, "tab")
	m = typed(m, "frontend")
	m, cmd := press(m, "enter")
	if m.pform != nil || cmd == nil {
		t.Fatalf("enter must close the form and run: %+v", m.pform)
	}
	m = settleAll(m, cmd)
	want := "create {Name:newp Flat:false Repos:[{URL:git@example.invalid:o/a.git Dir: Copy:[.env cfg/*.json]} {URL:https://example.invalid/o/b Dir:frontend Copy:[]}]}"
	if len(f.calls) != 1 || f.calls[0] != want {
		t.Errorf("calls = %q\nwant    %q", f.calls, want)
	}
}

func TestAddProjectFormRules(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	m, _ = press(m, "a", "enter")
	if m.pform == nil || !strings.Contains(m.pform.err, "name") {
		t.Fatalf("empty name: %+v", m.pform)
	}
	m = typed(m, "p")
	m, _ = press(m, "tab", "space", "ctrl+a", "enter") // flat with two repos
	if m.pform == nil || !strings.Contains(m.pform.err, "exactly one repo") {
		t.Fatalf("flat with two repos: %+v", m.pform)
	}
	m, _ = press(m, "tab", "ctrl+x") // focus the first repo's url, drop it
	if len(m.pform.repos) != 1 {
		t.Errorf("ctrl+x should drop the focused repo, have %d", len(m.pform.repos))
	}
	m, _ = press(m, "ctrl+x")
	if len(m.pform.repos) != 1 {
		t.Error("the last repo cannot be dropped")
	}
}

func TestProjectViewIgnoresTmuxKeysAndMouse(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	m.w, m.h = 100, 30
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	if next.(model).focus != focusSidebar {
		t.Error("alt+s must not move focus to a hidden tmux")
	}
	next, _ = m.Update(tea.MouseMsg{X: sidebarW + 5, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if next.(model).focus != focusSidebar {
		t.Error("a click on the details page must not focus tmux")
	}
}

func TestProjectsViewRendersWholeScreen(t *testing.T) {
	m := openProjects(t, &fakeProjects{infos: stackInfos()}, new([]string))
	for _, size := range [][2]int{{100, 30}, {60, 12}, {30, 5}} {
		m.w, m.h = size[0], size[1]
		lines := strings.Split(m.View(), "\n")
		if len(lines) != m.h {
			t.Errorf("%v: %d lines, want %d", size, len(lines), m.h)
		}
	}
	m.w, m.h = 100, 30
	m, _ = press(m, "n")
	if !strings.Contains(ansiText(m.View()), "new task in api-stack") {
		t.Error("the task form must draw over the details page")
	}
	m, _ = press(m, "esc", "a")
	if !strings.Contains(ansiText(m.View()), "add project") {
		t.Error("the project form must draw over the details page")
	}
}

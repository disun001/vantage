package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// noBlanks drops all whitespace from typed text: no field of these forms has
// a space in it (branch names, URLs, directory names, globs).
func noBlanks(s string) string { return strings.Join(strings.Fields(s), "") }

// typeInto applies a text-editing key to s. Other keys do nothing.
func typeInto(s *string, k tea.KeyMsg) {
	switch k.Type {
	case tea.KeyBackspace:
		if r := []rune(*s); len(r) > 0 {
			*s = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		*s = ""
	case tea.KeyRunes:
		*s += noBlanks(string(k.Runes))
	}
}

// fieldLine draws one form line: a label, then the text with a cursor when the
// line has focus.
func fieldLine(label, text string, focus bool, labelW, inner int) string {
	room := max(1, inner-labelW-1)
	if r := []rune(text); len(r) > room {
		text = "…" + string(r[len(r)-room+1:])
	}
	if focus {
		return curStyle.Render(fit(label, labelW)) + fit(text+cursorStyle.Render(" "), inner-labelW)
	}
	return fit(dimStyle.Render(fit(label, labelW))+text, inner)
}

// taskForm is the add-task pop-up: the task name, then one line per repo to
// include, each with a base branch.
type taskForm struct {
	project string
	name    string
	repos   []taskFormRepo
	focus   int // 0 is the name, 1.. the repos
	err     string
}

type taskFormRepo struct {
	name    string
	include bool
	base    string // empty means the repo's default branch
}

func (m model) startTaskForm() model {
	if m.projInfo == nil || m.projInfo.Name != m.projCur || len(m.projInfo.Repos) == 0 {
		return m
	}
	f := &taskForm{project: m.projCur}
	for _, r := range m.projInfo.Repos {
		f.repos = append(f.repos, taskFormRepo{name: r.Name, include: true})
	}
	m.tform, m.notice = f, ""
	return m
}

func (m model) onTaskFormKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := *m.tform
	f.repos = append([]taskFormRepo(nil), f.repos...)
	m.tform = &f
	n := len(f.repos) + 1
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.tform = nil
	case tea.KeyEnter:
		return m.submitTaskForm()
	case tea.KeyTab, tea.KeyDown:
		f.focus = (f.focus + 1) % n
	case tea.KeyShiftTab, tea.KeyUp:
		f.focus = (f.focus + n - 1) % n
	case tea.KeySpace:
		if f.focus > 0 {
			f.repos[f.focus-1].include = !f.repos[f.focus-1].include
		}
	default:
		if f.focus == 0 {
			typeInto(&f.name, k)
		} else {
			typeInto(&f.repos[f.focus-1].base, k)
		}
	}
	return m, nil
}

func (m model) submitTaskForm() (tea.Model, tea.Cmd) {
	f := *m.tform
	var picks []TaskPick
	for _, r := range f.repos {
		if r.include {
			picks = append(picks, TaskPick{Repo: r.name, Base: r.base})
		}
	}
	switch {
	case f.name == "":
		f.focus, f.err = 0, "task name is required"
	case len(picks) == 0:
		f.err = "include at least one repo"
	default:
		m.tform, m.notice, m.okNotice = nil, "creating task…", "creating task…"
		return m, m.taskCmd(f.project, f.name, picks)
	}
	m.tform = &f
	return m, nil
}

func (m model) taskFormBox() []string {
	f := m.tform
	inner := max(10, min(48, m.w-6))
	rows := []string{popTitle("new task in " + f.project), ""}
	rows = append(rows, fieldLine("task", f.name, f.focus == 0, 6, inner), "")
	for i, r := range f.repos {
		box := "[ ] "
		if r.include {
			box = "[x] "
		}
		base := r.base
		if base == "" {
			base = "default branch"
		}
		line := box + fit(ansi.Truncate(r.name, 14, "…"), 15) + "from " + base
		switch {
		case f.focus == i+1:
			rows = append(rows, curStyle.Render(fit(ansi.Truncate(line, inner, "…"), inner)))
		case !r.include:
			rows = append(rows, fit(dimStyle.Render(ansi.Truncate(line, inner, "…")), inner))
		default:
			rows = append(rows, fit(ansi.Truncate(line, inner, "…"), inner))
		}
	}
	rows = append(rows, "")
	if f.err != "" {
		rows = append(rows, noticeStyle.Render(f.err))
	}
	rows = append(rows, dimStyle.Render("tab next · space include · type base"), dimStyle.Render("enter create · esc"))
	return framed(rows, inner)
}

// projForm is the add-project pop-up: name, flat, and one block of url, dir
// and copy per repo.
type projForm struct {
	name  string
	flat  bool
	repos []projFormRepo
	focus int // 0 name, 1 flat, then three per repo
	err   string
}

type projFormRepo struct{ url, dir, copy string }

func newProjForm() *projForm { return &projForm{repos: []projFormRepo{{}}} }

var projRepoLabels = [3]string{"url", "dir", "copy"}

func (f *projForm) fields() int { return 2 + 3*len(f.repos) }

// ptr is the text of field i, or nil for the flat toggle.
func (f *projForm) ptr(i int) *string {
	switch i {
	case 0:
		return &f.name
	case 1:
		return nil
	}
	r := &f.repos[(i-2)/3]
	return []*string{&r.url, &r.dir, &r.copy}[(i-2)%3]
}

func (f projForm) def() ProjectDef {
	d := ProjectDef{Name: f.name, Flat: f.flat}
	for _, r := range f.repos {
		rd := RepoDef{URL: r.url, Dir: r.dir}
		for _, c := range strings.Split(r.copy, ",") {
			if c = strings.TrimSpace(c); c != "" {
				rd.Copy = append(rd.Copy, c)
			}
		}
		d.Repos = append(d.Repos, rd)
	}
	return d
}

func (m model) onProjFormKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := *m.pform
	f.repos = append([]projFormRepo(nil), f.repos...)
	m.pform = &f
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.pform = nil
	case tea.KeyEnter:
		return m.submitProjForm()
	case tea.KeyTab, tea.KeyDown:
		f.focus = (f.focus + 1) % f.fields()
	case tea.KeyShiftTab, tea.KeyUp:
		f.focus = (f.focus + f.fields() - 1) % f.fields()
	case tea.KeyCtrlA:
		f.repos = append(f.repos, projFormRepo{})
		f.focus = f.fields() - 3
	case tea.KeyCtrlX:
		if f.focus >= 2 && len(f.repos) > 1 {
			i := (f.focus - 2) / 3
			f.repos = append(f.repos[:i], f.repos[i+1:]...)
			f.focus = min(f.focus, f.fields()-1)
		}
	case tea.KeySpace:
		if f.focus == 1 {
			f.flat = !f.flat
		}
	default:
		if p := f.ptr(f.focus); p != nil {
			typeInto(p, k)
		}
	}
	return m, nil
}

func (m model) submitProjForm() (tea.Model, tea.Cmd) {
	f := *m.pform
	switch {
	case f.name == "":
		f.focus, f.err = 0, "name is required"
	case f.flat && len(f.repos) != 1:
		f.focus, f.err = 1, "flat needs exactly one repo"
	default:
		d, s := f.def(), m.cfg.Projects
		m.pform, m.notice, m.okNotice = nil, "cloning "+d.Name+"…", "cloning "+d.Name+"…"
		return m, func() tea.Msg { return projCreatedMsg{name: d.Name, err: s.Create(d)} }
	}
	m.pform = &f
	return m, nil
}

func (m model) projFormBox() []string {
	f := m.pform
	inner := max(10, min(52, m.w-6))
	rows := []string{popTitle("add project"), ""}
	rows = append(rows, fieldLine("name", f.name, f.focus == 0, 7, inner))
	flat := "[ ] one repo, no nesting"
	if f.flat {
		flat = "[x] one repo, no nesting"
	}
	if f.focus == 1 {
		rows = append(rows, curStyle.Render(fit("flat   "+flat, inner)))
	} else {
		rows = append(rows, fit(dimStyle.Render("flat   ")+flat, inner))
	}
	for i, r := range f.repos {
		rows = append(rows, "", dimStyle.Render(strings.Repeat("─", 2)+" repo "+string(rune('1'+i))))
		for j, v := range []string{r.url, r.dir, r.copy} {
			rows = append(rows, fieldLine(projRepoLabels[j], v, f.focus == 2+3*i+j, 7, inner))
		}
	}
	rows = append(rows, "")
	if f.err != "" {
		rows = append(rows, noticeStyle.Render(f.err))
	}
	rows = append(rows, dimStyle.Render("tab next · ctrl+a add repo · ctrl+x drop"), dimStyle.Render("copy: comma-separated globs · enter save · esc"))
	return framed(rows, inner)
}

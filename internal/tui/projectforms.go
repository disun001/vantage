package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

// projForm is the add-project pop-up: name, flat, and one block of url and
// dir per repo.
type projForm struct {
	name  string
	flat  bool
	repos []projFormRepo
	focus int // 0 name, 1 flat, then two per repo
	err   string
}

type projFormRepo struct{ url, dir string }

func newProjForm() *projForm { return &projForm{repos: []projFormRepo{{}}} }

var projRepoLabels = [2]string{"url", "dir"}

func (f *projForm) fields() int { return 2 + 2*len(f.repos) }

// ptr is the text of field i, or nil for the flat toggle.
func (f *projForm) ptr(i int) *string {
	switch i {
	case 0:
		return &f.name
	case 1:
		return nil
	}
	r := &f.repos[(i-2)/2]
	return []*string{&r.url, &r.dir}[(i-2)%2]
}

func (f projForm) def() ProjectDef {
	d := ProjectDef{Name: f.name, Flat: f.flat}
	for _, r := range f.repos {
		d.Repos = append(d.Repos, RepoDef{URL: r.url, Dir: r.dir})
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
		f.focus = f.fields() - 2
	case tea.KeyCtrlX:
		if f.focus >= 2 && len(f.repos) > 1 {
			i := (f.focus - 2) / 2
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
		for j, v := range []string{r.url, r.dir} {
			rows = append(rows, fieldLine(projRepoLabels[j], v, f.focus == 2+2*i+j, 7, inner))
		}
	}
	rows = append(rows, "")
	if f.err != "" {
		rows = append(rows, noticeStyle.Render(f.err))
	}
	rows = append(rows, dimStyle.Render("tab next · ctrl+a add repo · ctrl+x drop"), dimStyle.Render("enter save · esc"))
	return framed(rows, inner)
}

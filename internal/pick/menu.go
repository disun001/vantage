package pick

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ErrCancelled is returned by Choose when the user quits without choosing.
var ErrCancelled = errors.New("cancelled")

// localLabel is the first menu entry. It cannot collide with a host alias
// because aliases never contain spaces.
const localLabel = "local shell"

// Choose shows the menu on out, reading keys from in (a terminal), and returns
// the user's choice. Keys: j/k, arrows or ctrl-n/p move; g/G jump; "/" starts
// a fuzzy filter; Enter chooses; Esc leaves the filter, then cancels.
func Choose(in *os.File, out io.Writer, hosts []string) (Choice, error) {
	m := newMenu(hosts)
	final, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return Choice{}, fmt.Errorf("menu: %w", err)
	}
	res, ok := final.(menu)
	if !ok {
		return Choice{}, errors.New("menu: unexpected final model")
	}
	if res.cancelled {
		return Choice{}, ErrCancelled
	}
	return res.choice, nil
}

type menu struct {
	items     []string // localLabel, then host aliases
	query     string
	filtering bool // typing into the filter
	shown     []string
	cursor    int
	height    int
	choice    Choice
	cancelled bool
}

func newMenu(hosts []string) menu {
	items := append([]string{localLabel}, hosts...)
	return menu{items: items, shown: items, height: 12}
}

func (m menu) Init() tea.Cmd { return nil }

func (m menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = max(msg.Height-3, 1) // title, filter line, hint
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m menu) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyCtrlC:
		m.cancelled = true
		return m, tea.Quit
	case tea.KeyUp, tea.KeyCtrlP:
		return m.move(-1), nil
	case tea.KeyDown, tea.KeyCtrlN:
		return m.move(1), nil
	case tea.KeyEnter:
		return m.accept()
	case tea.KeyEsc:
		return m.escape()
	}
	if m.filtering {
		return m.typing(k), nil
	}
	return m.normal(k)
}

func (m menu) normal(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "j":
		return m.move(1), nil
	case "k":
		return m.move(-1), nil
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(len(m.shown)-1, 0)
	case "/":
		m.filtering = true
	case "q":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m menu) typing(k tea.KeyMsg) menu {
	switch k.Type {
	case tea.KeyBackspace:
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		m.query += string(k.Runes)
	default:
		return m
	}
	m.shown = filter(m.query, m.items)
	m.cursor = 0
	return m
}

func (m menu) move(d int) menu {
	if n := len(m.shown); n > 0 {
		m.cursor = (m.cursor + d + n) % n
	}
	return m
}

func (m menu) accept() (tea.Model, tea.Cmd) {
	if len(m.shown) == 0 {
		return m, nil
	}
	if sel := m.shown[m.cursor]; sel != localLabel {
		m.choice = Choice{Alias: sel}
	}
	return m, tea.Quit
}

// escape leaves filter typing first, then clears the filter, then cancels, so
// Esc never throws away a choice by accident.
func (m menu) escape() (tea.Model, tea.Cmd) {
	switch {
	case m.filtering:
		m.filtering = false
	case m.query != "":
		m.query, m.shown, m.cursor = "", m.items, 0
	default:
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m menu) View() string {
	var b strings.Builder
	b.WriteString("split where?\n")
	switch {
	case m.filtering:
		fmt.Fprintf(&b, "/%s\x1b[7m \x1b[0m\n", m.query)
	case m.query != "":
		fmt.Fprintf(&b, "/%s\n", m.query)
	default:
		b.WriteString("\n")
	}
	start := 0
	if m.cursor >= m.height {
		start = m.cursor - m.height + 1
	}
	for i := start; i < len(m.shown) && i < start+m.height; i++ {
		if i == m.cursor {
			fmt.Fprintf(&b, "\x1b[7m> %s\x1b[0m\n", m.shown[i])
		} else {
			fmt.Fprintf(&b, "  %s\n", m.shown[i])
		}
	}
	if len(m.shown) == 0 {
		b.WriteString("  no match\n")
	}
	b.WriteString("\nj/k move  / filter  enter choose  esc back")
	return b.String()
}

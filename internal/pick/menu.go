package pick

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/tmux"
)

// ErrCancelled is returned by Choose when the user quits without choosing.
var ErrCancelled = errors.New("cancelled")

// Host is one ssh host the menu offers. Open lists the live panes already
// connected to it, shown as an indicator (the menu never switches to them).
type Host struct {
	Alias     string
	HostName  string
	User      string
	Port      string
	ProxyJump string
	Open      []tmux.PaneRef
}

// Context describes the split being made, for the local-shell detail.
type Context struct {
	Dir string // "h" or "v"
	CWD string
}

const (
	listWidth = 24 // columns for the host list
	maxOpen   = 4  // open panes shown before "+N more"
)

// Choose shows the menu on out, reading keys from in (a terminal), and returns
// the user's choice. Keys: j/k, arrows or ctrl-n/p move; g/G jump; "/" starts
// a fuzzy filter; Enter chooses; Esc leaves the filter, then cancels.
func Choose(in *os.File, out io.Writer, ctx Context, hosts []Host) (Choice, error) {
	m := newMenu(ctx, hosts)
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

// item is one menu row. Index 0 is always the local shell.
type item struct {
	local  bool
	host   Host
	search string // what the fuzzy filter matches against
}

type menu struct {
	ctx       Context
	items     []item
	query     string
	filtering bool  // typing into the filter
	shown     []int // indexes into items, in display order
	cursor    int
	height    int
	choice    Choice
	cancelled bool
}

func newMenu(ctx Context, hosts []Host) menu {
	items := []item{{local: true, search: "local shell"}}
	for _, h := range hosts {
		items = append(items, item{host: h, search: h.Alias + " " + h.HostName})
	}
	m := menu{ctx: ctx, items: items, height: 10}
	m.shown = m.refilter()
	return m
}

func (m menu) refilter() []int {
	texts := make([]string, len(m.items))
	for i, it := range m.items {
		texts[i] = it.search
	}
	return filter(m.query, texts)
}

func (m menu) Init() tea.Cmd { return nil }

func (m menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = max(msg.Height-3, 3) // filter line, blank, footer
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
	m.shown = m.refilter()
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
	if it := m.items[m.shown[m.cursor]]; !it.local {
		m.choice = Choice{Alias: it.host.Alias}
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
		m.query, m.cursor = "", 0
		m.shown = m.refilter()
	default:
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

const (
	reverse = "\x1b[7m"
	reset   = "\x1b[0m"
	dim     = "\x1b[2m"
	green   = "\x1b[32m"
	purple  = "\x1b[35m"
	accent  = "\x1b[34m"
)

func (m menu) View() string {
	var b strings.Builder
	switch {
	case m.filtering:
		fmt.Fprintf(&b, "%s/%s%s%s %s\n", accent, reset, m.query, reverse, reset)
	case m.query != "":
		fmt.Fprintf(&b, "%s/%s%s\n", accent, reset, m.query)
	default:
		b.WriteString(dim + "/ to filter" + reset + "\n")
	}

	start := 0
	if m.cursor >= m.height {
		start = m.cursor - m.height + 1
	}
	var detail []string
	if len(m.shown) > 0 {
		detail = m.detail(m.items[m.shown[m.cursor]])
	}
	rows := min(max(len(m.shown)-start, len(detail), 1), m.height)
	for row := 0; row < rows; row++ {
		i := start + row
		left := strings.Repeat(" ", listWidth)
		switch {
		case i < len(m.shown):
			left = m.listCell(i)
		case len(m.shown) == 0 && row == 0:
			left = dim + pad("  no match", listWidth) + reset
		}
		right := ""
		if row < len(detail) {
			right = detail[row]
		}
		fmt.Fprintf(&b, "%s %s│%s %s\n", left, dim, reset, right)
	}
	b.WriteString("\n" + dim + "j/k move  / filter  enter open  esc back" + reset)
	return b.String()
}

// listCell renders one list row at a fixed width, so the divider lines up.
func (m menu) listCell(i int) string {
	it := m.items[m.shown[i]]
	label := "local shell"
	mark := " "
	if !it.local {
		label = it.host.Alias
		if len(it.host.Open) > 0 {
			mark = green + "●" + "\x1b[39m"
		}
	}
	text := truncate(label, listWidth-3)
	cell := mark + " " + text + strings.Repeat(" ", max(listWidth-3-utf8.RuneCountInString(text), 0)) + " "
	if i == m.cursor {
		return reverse + cell + reset
	}
	return cell
}

func (m menu) detail(it item) []string {
	if it.local {
		dir := "stacked (below)"
		if m.ctx.Dir == "h" {
			dir = "side by side (right)"
		}
		return []string{
			accent + "local shell" + reset, "",
			kv("runs", "your shell"),
			kv("cwd", m.ctx.CWD),
			kv("split", dir),
		}
	}
	h := it.host
	lines := []string{accent + h.Alias + reset, ""}
	if h.HostName != "" {
		lines = append(lines, kv("host", h.HostName))
	}
	if h.User != "" {
		lines = append(lines, kv("user", h.User))
	}
	if h.Port != "" {
		lines = append(lines, kv("port", h.Port))
	}
	if h.ProxyJump != "" {
		lines = append(lines, kv("jump", purple+h.ProxyJump+reset))
	}
	lines = append(lines, kv("runs", "ssh "+h.Alias), "")
	return append(lines, openLines(h.Open)...)
}

func openLines(open []tmux.PaneRef) []string {
	if len(open) == 0 {
		return []string{dim + "not connected" + reset}
	}
	noun := "panes"
	if len(open) == 1 {
		noun = "pane"
	}
	lines := []string{fmt.Sprintf("%s●%s connected in %d %s", green, reset, len(open), noun)}
	for i, p := range open {
		if i == maxOpen {
			lines = append(lines, fmt.Sprintf("%s  +%d more%s", dim, len(open)-maxOpen, reset))
			break
		}
		lines = append(lines, fmt.Sprintf("  %s:%d %s%s%s", p.Session, p.Window, dim, p.WindowName, reset))
	}
	return lines
}

func kv(k, v string) string { return fmt.Sprintf("%s%-6s%s%s", dim, k, reset, v) }

func pad(s string, n int) string {
	return s + strings.Repeat(" ", max(n-utf8.RuneCountInString(s), 0))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

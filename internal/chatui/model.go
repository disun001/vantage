package chatui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/store"
)

const inputHeight = 3

type (
	eventMsg store.Event
	sentMsg  struct{ err error }
)

type model struct {
	ctx    context.Context
	drv    Driver
	title  string
	events []store.Event
	view   viewport.Model
	input  textarea.Model
	notice string // last send/interrupt failure
	w, h   int
	ready  bool
}

func newModel(ctx context.Context, drv Driver, title string, history []store.Event) model {
	in := textarea.New()
	in.Prompt = "│ "
	in.ShowLineNumbers = false
	in.SetHeight(inputHeight)
	in.Placeholder = "message"
	// Enter sends, so a newline needs a modified key.
	in.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	in.Focus()
	return model{ctx: ctx, drv: drv, title: title, events: history, input: in}
}

func (m model) Init() tea.Cmd { return textarea.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg.Width, msg.Height), nil
	case eventMsg:
		return m.appendEvent(store.Event(msg)), nil
	case sentMsg:
		if msg.err != nil {
			m.notice = msg.err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m.pass(msg)
}

func (m model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "ctrl+c":
		if busy(m.events) {
			return m, m.interrupt()
		}
		return m, tea.Quit
	case "ctrl+d":
		return m, tea.Quit
	case "esc":
		if busy(m.events) {
			return m, m.interrupt()
		}
		return m, nil
	case "enter":
		return m.submit()
	case "pgup", "pgdown", "ctrl+u", "ctrl+f":
		var cmd tea.Cmd
		m.view, cmd = m.view.Update(k)
		return m, cmd
	}
	return m.pass(k)
}

// pass hands a message to the input so typing, paste and cursor keys work.
func (m model) pass(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" || busy(m.events) {
		return m, nil
	}
	m.input.Reset()
	m.notice = ""
	drv, ctx := m.drv, m.ctx
	return m, func() tea.Msg { return sentMsg{err: drv.Send(ctx, text)} }
}

func (m model) interrupt() tea.Cmd {
	drv, ctx := m.drv, m.ctx
	return func() tea.Msg { return sentMsg{err: drv.Interrupt(ctx)} }
}

func (m model) appendEvent(e store.Event) model {
	follow := m.view.AtBottom()
	m.events = append(m.events, e)
	m.refresh(follow)
	return m
}

func (m model) resize(w, h int) model {
	m.w, m.h = w, h
	m.input.SetWidth(w)
	m.view.Width = w
	m.view.Height = max(h-inputHeight-2, 1) // header and status line
	m.ready = true
	m.refresh(true)
	return m
}

// refresh redraws the transcript, keeping the bottom in view when following.
func (m *model) refresh(follow bool) {
	if !m.ready {
		return
	}
	m.view.SetContent(render(m.events, m.w))
	if follow {
		m.view.GotoBottom()
	}
}

func (m model) View() string {
	if !m.ready {
		return ""
	}
	head := dimStyle.Render(oneLine(m.title+headerExtra(m.events), m.w))
	return head + "\n" + m.view.View() + "\n" + m.input.View() + "\n" + m.statusLine()
}

func headerExtra(events []store.Event) string {
	if s := sessionInfo(events); s != "" {
		return "  " + s
	}
	return ""
}

func (m model) statusLine() string {
	switch {
	case m.notice != "":
		return errStyle.Render(oneLine(m.notice, m.w))
	case busy(m.events):
		return dimStyle.Render("working…  esc interrupt")
	}
	return dimStyle.Render("enter send  alt+enter newline  pgup/pgdn scroll  ctrl+c quit")
}

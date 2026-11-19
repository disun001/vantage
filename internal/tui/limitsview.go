package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/limits"
)

const (
	limitBarW     = 8  // cells in the usage bar
	limitMinH     = 12 // below this height the block is dropped; the list matters more
	limitMissDim  = 2  // failed polls in a row before the numbers are drawn dim
	limitBarFull  = "▓"
	limitBarEmpty = "░"
)

// limitState is what the sidebar knows about the usage windows.
type limitState struct {
	snap   limits.Snapshot
	ok     bool // a probe has worked once; until then the block stays hidden
	misses int  // probes that failed since the last good one
}

// limitsMsg is the result of one probe.
type limitsMsg struct {
	snap limits.Snapshot
	err  error
}

// onLimits records a probe result.
func (s limitState) onLimits(msg limitsMsg) limitState {
	if msg.err != nil {
		s.misses++
		return s
	}
	return limitState{snap: msg.snap, ok: true}
}

// limitRows is the block above the "? keys" hint: one line per window, or
// nothing when the feature is off, has never worked, or the window is short.
func (m model) limitRows() []string {
	if m.cfg.Limits == nil || !m.limits.ok || m.h < limitMinH {
		return nil
	}
	now := time.Now()
	dim := m.limits.misses >= limitMissDim
	return []string{
		fit(limitLine("5h", m.limits.snap.FiveHour, now, dim), sidebarW),
		fit(limitLine("wk", m.limits.snap.SevenDay, now, dim), sidebarW),
	}
}

// limitLine draws "5h ▓▓░░░░░░ 24% ↻2h10m". A window whose reset time has
// passed is shown as "--": the old number belongs to a window that is gone.
func limitLine(label string, w limits.Window, now time.Time, dim bool) string {
	fg := mocSub
	if dim {
		fg = mocOverlay
	}
	if !w.Known || !w.ResetsAt.After(now) {
		return paint(fg, "").Render(fmt.Sprintf("%s %s  --", label, strings.Repeat(limitBarEmpty, limitBarW)))
	}
	used := min(max(w.Used, 0), 1)
	n := int(math.Round(used * limitBarW))
	bar := strings.Repeat(limitBarFull, n) + strings.Repeat(limitBarEmpty, limitBarW-n)
	pct := int(math.Round(used * 100))
	return paint(fg, "").Render(fmt.Sprintf("%s %s %3d%% ↻%s", label, bar, pct, countdown(w.ResetsAt.Sub(now))))
}

// countdown writes a duration as "4d2h", "2h10m" or "45m".
func countdown(d time.Duration) string {
	mins := int(d.Round(time.Minute) / time.Minute)
	switch {
	case mins >= 24*60:
		return fmt.Sprintf("%dd%dh", mins/(24*60), mins%(24*60)/60)
	case mins >= 60:
		return fmt.Sprintf("%dh%02dm", mins/60, mins%60)
	default:
		return fmt.Sprintf("%dm", max(mins, 0))
	}
}

// drawFoot fills the last lines of a sidebar: the notice, the limits block,
// then the "? keys" hint. Every view calls it so the bottom stays identical.
// dimOK draws a good-news notice dim instead of as an error.
func (m model) drawFoot(out []string, dimOK bool) {
	lim := m.limitRows()
	note := m.noticeLines()
	for i, l := range note {
		style := noticeStyle
		if dimOK && m.notice == m.okNotice {
			style = dimStyle
		}
		out[m.h-1-len(lim)-len(note)+i] = fit(style.Render(l), sidebarW)
	}
	for i, l := range lim {
		out[m.h-1-len(lim)+i] = l
	}
	out[m.h-1] = fit(hintStyle("? keys"), sidebarW)
}

type limitsTickMsg struct{}

// limitsInit probes once at start, then on every tick. Without Limits it
// does nothing.
func (m model) limitsInit() tea.Cmd {
	if m.cfg.Limits == nil {
		return nil
	}
	return tea.Batch(m.probeLimits(), m.limitsTick())
}

func (m model) limitsTick() tea.Cmd {
	every := m.cfg.LimitsEvery
	if every <= 0 {
		every = limits.DefaultInterval // a zero tick would spin
	}
	return tea.Tick(every, func(time.Time) tea.Msg { return limitsTickMsg{} })
}

// probeLimits runs the probe off the UI loop: it takes seconds.
func (m model) probeLimits() tea.Cmd {
	probe := m.cfg.Limits
	return func() tea.Msg {
		snap, err := probe()
		return limitsMsg{snap: snap, err: err}
	}
}

// updateLimits handles the probe's messages; ok is false for any other.
func (m model) updateLimits(msg tea.Msg) (model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case limitsMsg:
		m.limits = m.limits.onLimits(msg)
		return m, nil, true
	case limitsTickMsg:
		return m, tea.Batch(m.probeLimits(), m.limitsTick()), true
	}
	return m, nil, false
}

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disun001/vantage/internal/hostuse"
)

// hostInfoMsg carries what uses the host under the cursor, for the details page.
type hostInfoMsg struct {
	alias string
	use   hostuse.Usage
	err   error
}

// hostInfo is the usage of one host, once loaded.
type hostInfo struct {
	alias string
	use   hostuse.Usage
	err   error
}

// loadHostInfo looks up the workspaces and open panes that use the host under
// the cursor. Without a usage finder the page shows the config only.
func (m model) loadHostInfo() tea.Cmd {
	u := m.cfg.HostUse
	if u == nil || len(m.hosts) == 0 {
		return nil
	}
	alias := m.hosts[m.hostPos()].Alias
	return func() tea.Msg {
		use, err := u.Find(alias)
		return hostInfoMsg{alias: alias, use: use, err: err}
	}
}

func (m model) onHostInfo(msg hostInfoMsg) model {
	m.hostInfo = &hostInfo{alias: msg.alias, use: msg.use, err: msg.err}
	return m
}

// hostDetails draws the right pane of the hosts view: the ssh options of the
// host under the cursor, the saved workspaces that use it and its open panes.
func (m model) hostDetails() []string {
	w, h := m.termW(), m.innerH()
	out := make([]string, h)
	for i := range out {
		out[i] = strings.Repeat(" ", w)
	}
	if len(m.hosts) == 0 {
		msg := "no hosts: a adds one"
		out[h/2] = fit(strings.Repeat(" ", max(0, (w-len(msg))/2))+dimStyle.Render(msg), w)
		return out
	}
	host := m.hosts[m.hostPos()]
	field := func(label, val string) string {
		if val == "" {
			val = dimStyle.Render("-")
		}
		return " " + dimStyle.Render(fit(label, 7)) + val
	}
	head := []string{
		" " + curStyle.Render(host.Alias), "",
		field("host", host.HostName),
		field("user", host.User),
		field("port", host.Port),
		field("jump", host.ProxyJump),
		field("key", host.Identity),
		"",
	}
	info := m.hostInfo
	switch {
	case m.cfg.HostUse == nil:
	case info == nil || info.alias != host.Alias:
		head = append(head, " "+dimStyle.Render("loading…"))
	default:
		if info.err != nil {
			head = append(head, " "+noticeStyle.Render(oneLine(info.err)))
		}
		head = append(head, " "+dimStyle.Render("open panes"))
		if len(info.use.Panes) == 0 {
			head = append(head, "   "+dimStyle.Render("not connected"))
		}
		for _, p := range info.use.Panes {
			head = append(head, "   "+p.Session+":"+fmt.Sprint(p.Window)+" "+p.WindowName)
		}
		head = append(head, "", " "+dimStyle.Render("saved workspaces"))
		if len(info.use.Workspaces) == 0 {
			head = append(head, "   "+dimStyle.Render("none"))
		}
		for _, n := range info.use.Workspaces {
			head = append(head, "   "+n)
		}
	}
	for i := 0; i < len(head) && i < h-1; i++ {
		out[i] = fit(head[i], w)
	}
	out[h-1] = fit(" "+dimStyle.Render("⏎ connect · v/s split · x close panes · a add · e edit · d delete"), w)
	return out
}

// Package sshconfig lists the hosts in an ssh_config file. It is read-only and
// exposes only what the picker displays (alias, address, user, port, jump
// host). It never returns key paths or other option values.
package sshconfig

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/kevinburke/ssh_config"

	"github.com/disun001/vantage/internal/workspace"
)

// Host is one concrete host entry. Fields other than Alias are empty when the
// config does not set them (wildcard `Host *` blocks are applied).
type Host struct {
	Alias     string
	HostName  string
	User      string
	Port      string
	ProxyJump string
}

// Hosts returns the concrete hosts in r, in file order, without duplicates.
// Wildcard and negated patterns (`*`, `?`, `!`) are skipped because they cannot
// be connected to. Aliases that fail workspace.ValidSSHAlias are skipped too,
// so every Alias is safe to put in a command line.
func Hosts(r io.Reader) ([]Host, error) {
	cfg, err := ssh_config.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("parse ssh config: %w", err)
	}
	var out []Host
	seen := map[string]bool{}
	for _, h := range cfg.Hosts {
		for _, p := range h.Patterns {
			a := p.String()
			if strings.ContainsAny(a, "*?!") || !workspace.ValidSSHAlias(a) || seen[a] {
				continue
			}
			seen[a] = true
			out = append(out, Host{
				Alias:     a,
				HostName:  get(cfg, a, "HostName"),
				User:      get(cfg, a, "User"),
				Port:      get(cfg, a, "Port"),
				ProxyJump: get(cfg, a, "ProxyJump"),
			})
		}
	}
	return out, nil
}

// get returns a config value, or "" when unset or unreadable. Display fields
// are optional, so a lookup problem must not hide the host.
func get(cfg *ssh_config.Config, alias, key string) string {
	v, err := cfg.Get(alias, key)
	if err != nil {
		return ""
	}
	return v
}

// Load is Hosts for a file. A missing file is not an error: it means no hosts.
func Load(path string) ([]Host, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ssh config: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only: a close error loses nothing
	return Hosts(f)
}

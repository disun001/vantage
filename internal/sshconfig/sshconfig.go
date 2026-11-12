// Package sshconfig lists the host aliases in an ssh_config file. It is
// read-only and exposes aliases only: no keys, users, or other option values.
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

// Hosts returns the concrete host aliases in r, in file order, without
// duplicates. Wildcard and negated patterns (`*`, `?`, `!`) are skipped because
// they cannot be connected to. Aliases that fail workspace.ValidSSHAlias are
// skipped too, so every result is safe to put in a command line.
func Hosts(r io.Reader) ([]string, error) {
	cfg, err := ssh_config.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("parse ssh config: %w", err)
	}
	var out []string
	seen := map[string]bool{}
	for _, h := range cfg.Hosts {
		for _, p := range h.Patterns {
			a := p.String()
			if strings.ContainsAny(a, "*?!") || !workspace.ValidSSHAlias(a) || seen[a] {
				continue
			}
			seen[a] = true
			out = append(out, a)
		}
	}
	return out, nil
}

// Load is Hosts for a file. A missing file is not an error: it means no hosts.
func Load(path string) ([]string, error) {
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

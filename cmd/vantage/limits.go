package main

import (
	"context"
	"fmt"
	"time"

	"github.com/disun001/vantage/internal/limits"
	"github.com/disun001/vantage/internal/workspace"
)

// tuiLimits is the usage block's wiring: a nil probe leaves it off.
type tuiLimits struct {
	probe func() (limits.Snapshot, error)
	every time.Duration
	stop  func() // cancels a probe in flight; never nil
	warn  string // why the block is off, when config.toml is bad
}

// limitsConfig reads config.toml and returns the probe only when the user
// opted in with [limits] enabled = true. A bad config.toml turns it off with
// a warning instead of failing.
func limitsConfig() (tuiLimits, error) {
	dir, err := workspace.ConfigDir()
	if err != nil {
		return tuiLimits{stop: func() {}}, fmt.Errorf("config dir: %w", err)
	}
	cfg, err := limits.LoadConfig(dir)
	if err != nil {
		// A cosmetic feature must not stop vantage: turn it off and say why.
		return tuiLimits{stop: func() {}, warn: "limits off: " + err.Error()}, nil
	}
	if !cfg.Enabled {
		return tuiLimits{stop: func() {}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	return tuiLimits{stop: cancel, probe: func() (limits.Snapshot, error) { return limits.Probe(ctx) }, every: cfg.Interval}, nil
}

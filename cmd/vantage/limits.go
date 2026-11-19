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
}

// limitsConfig reads config.toml and returns the probe only when the user
// opted in with [limits] enabled = true.
func limitsConfig() (tuiLimits, error) {
	dir, err := workspace.ConfigDir()
	if err != nil {
		return tuiLimits{stop: func() {}}, fmt.Errorf("config dir: %w", err)
	}
	cfg, err := limits.LoadConfig(dir)
	if err != nil {
		return tuiLimits{stop: func() {}}, err
	}
	if !cfg.Enabled {
		return tuiLimits{stop: func() {}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	return tuiLimits{stop: cancel, probe: func() (limits.Snapshot, error) { return limits.Probe(ctx) }, every: cfg.Interval}, nil
}

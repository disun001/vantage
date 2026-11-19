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
}

// limitsConfig reads config.toml and returns the probe only when the user
// opted in with [limits] enabled = true.
func limitsConfig() (tuiLimits, error) {
	dir, err := workspace.ConfigDir()
	if err != nil {
		return tuiLimits{}, fmt.Errorf("config dir: %w", err)
	}
	cfg, err := limits.LoadConfig(dir)
	if err != nil {
		return tuiLimits{}, err
	}
	if !cfg.Enabled {
		return tuiLimits{}, nil
	}
	return tuiLimits{probe: func() (limits.Snapshot, error) { return limits.Probe(context.Background()) }, every: cfg.Interval}, nil
}

package limits

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultInterval is how often the probe runs.
	DefaultInterval = 5 * time.Minute
	// MinInterval is the floor: each probe is a real request.
	MinInterval = 2 * time.Minute
)

// Config is the [limits] table of config.toml. The probe runs a program and
// makes a network call, so it is off unless the user turns it on.
type Config struct {
	Enabled  bool
	Interval time.Duration
}

type fileConfig struct {
	Limits struct {
		Enabled  bool   `toml:"enabled"`
		Interval string `toml:"interval"`
	} `toml:"limits"`
}

// LoadConfig reads dir/config.toml. A missing file means disabled. An interval
// below MinInterval is raised to it.
func LoadConfig(dir string) (Config, error) {
	cfg := Config{Interval: DefaultInterval}
	var f fileConfig
	path := filepath.Join(dir, "config.toml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &f); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.Enabled = f.Limits.Enabled
	if f.Limits.Interval != "" {
		d, err := time.ParseDuration(f.Limits.Interval)
		if err != nil {
			return cfg, fmt.Errorf("%s: limits.interval: %w", path, err)
		}
		cfg.Interval = max(d, MinInterval)
	}
	return cfg, nil
}

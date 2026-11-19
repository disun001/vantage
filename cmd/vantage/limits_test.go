package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLimitsConfigBadFileWarns(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VANTAGE_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[limits\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lim, err := limitsConfig()
	if err != nil {
		t.Fatalf("a bad config must not fail: %v", err)
	}
	if lim.probe != nil || !strings.HasPrefix(lim.warn, "limits off:") {
		t.Errorf("want off with a warning, got probe=%v warn=%q", lim.probe != nil, lim.warn)
	}
	lim.stop()
}

func TestLimitsConfigOffByDefault(t *testing.T) {
	t.Setenv("VANTAGE_CONFIG_DIR", t.TempDir())
	lim, err := limitsConfig()
	if err != nil || lim.probe != nil || lim.warn != "" {
		t.Errorf("want off and quiet, got %+v err %v", lim, err)
	}
	lim.stop()
}

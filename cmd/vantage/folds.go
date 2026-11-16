package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disun001/vantage/internal/workspace"
)

// foldsPath is the file that remembers which sessions the sidebar has folded.
func foldsPath() (string, error) {
	dir, err := workspace.StateDir()
	if err != nil {
		return "", fmt.Errorf("state directory: %w", err)
	}
	return filepath.Join(dir, "folds"), nil
}

// loadFolds reads the folded session names, one per line. A missing file
// means nothing is folded.
func loadFolds() ([]string, error) {
	path, err := foldsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read folds: %w", err)
	}
	var names []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	return names, nil
}

// saveFolds writes the folded session names, one per line.
func saveFolds(names []string) error {
	path, err := foldsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	body := strings.Join(names, "\n")
	if body != "" {
		body += "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write folds: %w", err)
	}
	return nil
}

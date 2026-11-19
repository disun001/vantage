// Package limits reads how much of Claude's 5-hour and weekly usage windows
// is spent. It owns the stream-json parsing and the probe that runs the
// claude CLI; it knows nothing about the TUI and never touches credentials.
package limits

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// ErrNoLimits means the output held no usage windows, as with an API-key
// account that has no subscription limits.
var ErrNoLimits = errors.New("no usage windows in the output")

// Window is one usage window.
type Window struct {
	Used     float64   // fraction spent, 0 to 1
	ResetsAt time.Time // when the window rolls over
	Known    bool
}

// Snapshot is both windows at one moment.
type Snapshot struct {
	FiveHour Window
	SevenDay Window
}

// event is the part of a stream-json line this package reads. The CLI emits
// one rate_limit_event per request.
type event struct {
	Type string `json:"type"`
	Info struct {
		Windows map[string]struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    int64   `json:"resetsAt"`
		} `json:"unifiedWindows"`
	} `json:"rate_limit_info"`
}

// ErrOutOfRange means a value is not in the unit this package assumes: a
// utilization that is a fraction near 0 to 1, a reset time in epoch seconds.
// Drawing it anyway would show a wrong number with full confidence.
var ErrOutOfRange = errors.New("usage value out of range")

const (
	maxUtilization = 2               // overage can pass 1; a percent such as 24 cannot be mistaken for it
	maxEpochSecs   = 100_000_000_000 // past year 5138; larger means milliseconds
)

// toWindow checks one window and converts it.
func toWindow(utilization float64, resetsAt int64) (Window, error) {
	if utilization < 0 || utilization > maxUtilization || resetsAt <= 0 || resetsAt > maxEpochSecs {
		return Window{}, fmt.Errorf("%w: utilization %v, resetsAt %d", ErrOutOfRange, utilization, resetsAt)
	}
	return Window{Used: utilization, ResetsAt: time.Unix(resetsAt, 0), Known: true}, nil
}

// Parse reads claude's --output-format stream-json output and returns the
// windows of the last rate_limit_event. Lines that are not JSON (a banner, a
// warning) are skipped. It returns ErrNoLimits when there is no event, and
// ErrOutOfRange when a value is not in the expected unit. Lines are read
// whole because the init line is far longer than a bufio.Scanner line.
func Parse(r io.Reader) (Snapshot, error) {
	var snap Snapshot
	found := false
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var ev event
			if json.Unmarshal(line, &ev) == nil && ev.Type == "rate_limit_event" {
				ok, perr := apply(&snap, ev)
				if perr != nil {
					return Snapshot{}, perr
				}
				found = found || ok
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("read stream: %w", err)
		}
	}
	if !found {
		return Snapshot{}, ErrNoLimits
	}
	return snap, nil
}

// apply copies the windows of one event into snap and reports whether it
// held any.
func apply(snap *Snapshot, ev event) (bool, error) {
	found := false
	for name, dst := range map[string]*Window{"five_hour": &snap.FiveHour, "seven_day": &snap.SevenDay} {
		w, ok := ev.Info.Windows[name]
		if !ok {
			continue
		}
		win, err := toWindow(w.Utilization, w.ResetsAt)
		if err != nil {
			return false, fmt.Errorf("%s: %w", name, err)
		}
		*dst, found = win, true
	}
	return found, nil
}

// Package limits reads how much of Claude's 5-hour and weekly usage windows
// is spent. It owns the stream-json parsing and the probe that runs the
// claude CLI; it knows nothing about the TUI and never touches credentials.
package limits

import (
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

// Parse reads claude's --output-format stream-json output and returns the
// windows of the last rate_limit_event. It returns ErrNoLimits when there is
// none. A json.Decoder is used because the init line is far longer than a
// bufio.Scanner line.
func Parse(r io.Reader) (Snapshot, error) {
	var snap Snapshot
	found := false
	dec := json.NewDecoder(r)
	for {
		var ev event
		err := dec.Decode(&ev)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("parse stream: %w", err)
		}
		if ev.Type != "rate_limit_event" {
			continue
		}
		if w, ok := ev.Info.Windows["five_hour"]; ok {
			snap.FiveHour = Window{Used: w.Utilization, ResetsAt: time.Unix(w.ResetsAt, 0), Known: true}
			found = true
		}
		if w, ok := ev.Info.Windows["seven_day"]; ok {
			snap.SevenDay = Window{Used: w.Utilization, ResetsAt: time.Unix(w.ResetsAt, 0), Known: true}
			found = true
		}
	}
	if !found {
		return Snapshot{}, ErrNoLimits
	}
	return snap, nil
}

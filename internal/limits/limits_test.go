package limits

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const stream = `{"type":"system","subtype":"init","tools":["Bash"]}
{"type":"assistant","message":{"content":[]}}
{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","unifiedWindows":{"five_hour":{"utilization":0.24,"resetsAt":1795065000},"seven_day":{"utilization":0.03,"resetsAt":1795615200}}}}
{"type":"result","total_cost_usd":0.0004}
`

func TestParse(t *testing.T) {
	s, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if !s.FiveHour.Known || s.FiveHour.Used != 0.24 || s.FiveHour.ResetsAt.Unix() != 1795065000 {
		t.Errorf("five hour = %+v", s.FiveHour)
	}
	if !s.SevenDay.Known || s.SevenDay.Used != 0.03 || s.SevenDay.ResetsAt.Unix() != 1795615200 {
		t.Errorf("seven day = %+v", s.SevenDay)
	}
}

func TestParseNoLimits(t *testing.T) {
	for name, in := range map[string]string{
		"empty":     "",
		"no event":  `{"type":"result"}`,
		"no window": `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`,
	} {
		if _, err := Parse(strings.NewReader(in)); !errors.Is(err, ErrNoLimits) {
			t.Errorf("%s: err = %v, want ErrNoLimits", name, err)
		}
	}
}

func TestParseBadJSON(t *testing.T) {
	if _, err := Parse(strings.NewReader("{not json")); err == nil || errors.Is(err, ErrNoLimits) {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name, body string
		want       Config
		wantErr    bool
	}{
		{"missing file", "", Config{Interval: DefaultInterval}, false},
		{"enabled", "[limits]\nenabled = true\n", Config{Enabled: true, Interval: DefaultInterval}, false},
		{"interval", "[limits]\nenabled = true\ninterval = \"10m\"\n", Config{Enabled: true, Interval: 10 * time.Minute}, false},
		{"floor", "[limits]\nenabled = true\ninterval = \"30s\"\n", Config{Enabled: true, Interval: MinInterval}, false},
		{"bad interval", "[limits]\ninterval = \"soon\"\n", Config{}, true},
		{"bad toml", "[limits\n", Config{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.body != "" {
				if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := LoadConfig(dir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

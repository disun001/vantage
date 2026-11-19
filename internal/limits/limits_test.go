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

func TestParseSkipsNonJSONLines(t *testing.T) {
	in := "warning: something\n{not json\n" + stream + "trailing banner"
	s, err := Parse(strings.NewReader(in))
	if err != nil || !s.FiveHour.Known {
		t.Fatalf("snap %+v, err %v", s, err)
	}
}

func TestParseOutOfRange(t *testing.T) {
	win := func(u, r string) string {
		return `{"type":"rate_limit_event","rate_limit_info":{"unifiedWindows":{"five_hour":{"utilization":` + u + `,"resetsAt":` + r + `}}}}`
	}
	for name, in := range map[string]string{
		"percent not fraction": win("24", "1795065000"),
		"negative":             win("-0.1", "1795065000"),
		"milliseconds":         win("0.2", "1795065000000"),
		"zero reset":           win("0.2", "0"),
	} {
		if _, err := Parse(strings.NewReader(in)); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("%s: err = %v, want ErrOutOfRange", name, err)
		}
	}
	if _, err := Parse(strings.NewReader(win("1.05", "1795065000"))); err != nil {
		t.Errorf("overage above 1 is valid: %v", err)
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

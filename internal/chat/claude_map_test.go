package chat

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func fixtureLines(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// The fixture is a real captured transcript: hooks, init, tool_use,
// rate_limit, tool_result, text, commands_changed, result.
func TestParseRealToolUseTranscript(t *testing.T) {
	var kinds []string
	var evs []Event
	for _, l := range fixtureLines(t, "tooluse.jsonl") {
		for _, e := range parseLine(l) {
			kinds = append(kinds, e.Kind)
			evs = append(evs, e)
		}
	}
	want := []string{KindSession, KindToolUse, KindToolResult, KindAssistantMsg, KindTurnEnd}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	if evs[0].SessionID == "" || evs[0].AuthSource != "subscription" || evs[0].Model == "" {
		t.Fatalf("session event %+v", evs[0])
	}
	if !strings.HasPrefix(evs[1].Text, "Read ") || !strings.Contains(evs[1].Text, "a.txt") {
		t.Fatalf("tool_use text %q", evs[1].Text)
	}
	if !strings.Contains(evs[2].Text, "hello") {
		t.Fatalf("tool_result text %q", evs[2].Text)
	}
	for _, e := range evs {
		if e.Raw == "" {
			t.Fatalf("%s lost its raw line", e.Kind)
		}
	}
	if evs[4].IsError {
		t.Fatal("turn_end must not be an error")
	}
}

func TestParseEdgeCases(t *testing.T) {
	tests := []struct {
		name, in, kind string
		none           bool
	}{
		{name: "bad json", in: "not json", kind: KindError},
		{name: "unknown type", in: `{"type":"brand_new"}`, kind: KindUnknown},
		{name: "unknown system subtype", in: `{"type":"system","subtype":"future"}`, kind: KindUnknown},
		{name: "hook noise ignored", in: `{"type":"system","subtype":"hook_started"}`, none: true},
		{name: "rate limit ignored", in: `{"type":"rate_limit_event"}`, none: true},
		{name: "error result", in: `{"type":"result","is_error":true,"result":"boom"}`, kind: KindTurnEnd},
		{name: "empty text skipped", in: `{"type":"assistant","message":{"content":[{"type":"text","text":" "}]}}`, none: true},
		{name: "thinking skipped", in: `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"x"}]}}`, none: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evs := parseLine(tt.in)
			if tt.none {
				if len(evs) != 0 {
					t.Fatalf("want no events, got %+v", evs)
				}
				return
			}
			if len(evs) != 1 || evs[0].Kind != tt.kind {
				t.Fatalf("got %+v, want kind %s", evs, tt.kind)
			}
		})
	}
}

func TestToolResultTruncatedRawKept(t *testing.T) {
	long := strings.Repeat("x", 2000)
	in := `{"type":"user","message":{"content":[{"type":"tool_result","content":"` + long + `"}]}}`
	evs := parseLine(in)
	if len(evs) != 1 || len([]rune(evs[0].Text)) > toolResultMax+1 || evs[0].Raw != in {
		t.Fatalf("got text len %d raw kept %v", len(evs[0].Text), evs[0].Raw == in)
	}
}

func TestAPIKeyStripped(t *testing.T) {
	env := []string{"PATH=/bin", "ANTHROPIC_API_KEY=secret", "HOME=/h"}
	got := Claude{Env: env}.childEnv()
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got := (Claude{Env: env, AllowAPIKey: true}).childEnv(); len(got) != 3 {
		t.Fatalf("allow: got %v", got)
	}
}

func TestClaudeArgs(t *testing.T) {
	a := strings.Join(claudeArgs(nil, "abc"), " ")
	for _, want := range []string{"-p", "--permission-mode plan", "--resume abc", "stream-json"} {
		if !strings.Contains(a, want) {
			t.Fatalf("%q missing from %q", want, a)
		}
	}
	if strings.Contains(a, "--bare") || strings.Contains(strings.Join(claudeArgs(nil, ""), " "), "--resume") {
		t.Fatalf("bad args %q", a)
	}
}

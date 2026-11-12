package capture

import (
	"strings"
	"testing"
)

func TestCompactLayout(t *testing.T) {
	two := `{"V":2,"L":{"t":"h","w":120,"h":30,"x":0,"y":0,"c":[{"t":"p","w":60,"h":30,"x":0,"y":0,"l":0,"i":0,"I":"%0"},{"t":"p","w":59,"h":30,"x":61,"y":0,"a":true,"i":1,"I":"%1"}]}}`
	one := `{"V":2,"L":{"t":"p","w":200,"h":50,"x":0,"y":0,"a":true,"i":0,"I":"%3"}}`
	for _, tt := range []struct {
		name, in string
		body     string // expected layout after the checksum, "" for none
	}{
		{"side by side", two, "120x30,0,0{60x30,0,0,0,59x30,61,0,1}"},
		{"single pane", one, ""},
		{"classic passes through", "abcd,80x24,0,0,0", "\x00classic"},
		{"garbage passes through", "{not json", "\x00garbage"},
	} {
		got := compactLayout(tt.in)
		switch tt.body {
		case "\x00classic", "\x00garbage":
			if got != tt.in {
				t.Errorf("%s: got %q, want input back", tt.name, got)
			}
		case "":
			if got != "" {
				t.Errorf("%s: got %q, want empty", tt.name, got)
			}
		default:
			_, body, ok := strings.Cut(got, ",")
			if !ok || body != tt.body || len(got)-len(body) != 5 {
				t.Errorf("%s: got %q, want <csum>,%s", tt.name, got, tt.body)
			}
			if want := checksum(tt.body); got[:4] != strings.ToLower(formatCsum(want)) {
				t.Errorf("%s: checksum %q != %04x", tt.name, got[:4], want)
			}
		}
	}
}

func formatCsum(c uint16) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[c>>12], hex[c>>8&15], hex[c>>4&15], hex[c&15]})
}

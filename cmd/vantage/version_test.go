package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "1.2.3"

	for _, arg := range []string{"version", "--version"} {
		var out, errOut bytes.Buffer
		if code := run([]string{arg}, &out, &errOut); code != 0 {
			t.Fatalf("%s: code=%d err=%q", arg, code, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != "vantage 1.2.3" {
			t.Errorf("%s: got %q, want %q", arg, got, "vantage 1.2.3")
		}
	}
}

func TestResolveVersion(t *testing.T) {
	for _, tt := range []struct{ stamped, module, want string }{
		{"0.1.0", "v9.9.9", "0.1.0"}, // release build: ldflags win
		{"dev", "v0.2.0", "v0.2.0"},  // go install ...@v0.2.0
		{"dev", "(devel)", "dev"},    // go build in a checkout
		{"dev", "", "dev"},           // no build info
	} {
		if got := resolveVersion(tt.stamped, tt.module); got != tt.want {
			t.Errorf("resolveVersion(%q, %q) = %q, want %q", tt.stamped, tt.module, got, tt.want)
		}
	}
}

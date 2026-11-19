package limits

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// probeTimeout bounds one probe so a hung CLI cannot pile up processes.
const probeTimeout = 60 * time.Second

// probeArgs makes the cheapest possible request: the small model, no tools,
// no MCP servers, no slash commands and no saved session. The output must be
// stream-json, the only format that carries the rate_limit_event. Nothing in
// here comes from user input.
var probeArgs = []string{
	"-p", ".",
	"--model", "haiku",
	"--tools", "",
	"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
	"--disable-slash-commands",
	"--setting-sources", "", // user hooks would run on every probe
	"--no-session-persistence",
	"--system-prompt", ".",
	"--output-format", "stream-json", "--verbose",
}

// Probe runs the claude CLI once in an empty temporary directory and returns
// the usage windows it reports. The directory keeps project files and
// CLAUDE.md out of the request.
func Probe(ctx context.Context) (Snapshot, error) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return Snapshot{}, fmt.Errorf("claude not found: %w", err)
	}
	dir, err := os.MkdirTemp("", "vantage-limits-")
	if err != nil {
		return Snapshot{}, fmt.Errorf("temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, probeArgs...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	runErr := cmd.Run()
	// The windows may be in the output even when the exit code is not 0.
	snap, parseErr := Parse(&out)
	if parseErr == nil {
		return snap, nil
	}
	if runErr != nil {
		return Snapshot{}, fmt.Errorf("run claude: %w: %s", runErr, firstLine(stderr.String()))
	}
	return Snapshot{}, parseErr
}

// firstLine keeps an error message to one short line.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line[:min(len(line), 200)]
}

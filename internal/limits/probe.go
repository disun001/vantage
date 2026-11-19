package limits

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Snapshot{}, fmt.Errorf("run claude: %w", err)
	}
	return Parse(&out)
}

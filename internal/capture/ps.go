package capture

import (
	"fmt"
	"os/exec"
	"strings"
)

// PS lists the foreground command lines on a tty using `ps`. The same
// invocation works on macOS and Linux: `-t` takes the tty name without /dev/,
// and a "+" in the STAT column marks the foreground process group.
func PS(tty string) ([]string, error) {
	name, ok := strings.CutPrefix(tty, "/dev/")
	if !ok || name == "" || strings.HasPrefix(name, "-") {
		return nil, fmt.Errorf("unexpected tty %q", tty)
	}
	out, err := exec.Command("ps", "-t", name, "-o", "stat=,args=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		stat, args, found := strings.Cut(strings.TrimSpace(l), " ")
		if found && strings.Contains(stat, "+") {
			lines = append(lines, strings.TrimSpace(args))
		}
	}
	return lines, nil
}

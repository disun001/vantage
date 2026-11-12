package capture

import (
	"path/filepath"
	"strings"

	"github.com/disun001/vantage/internal/workspace"
)

// sshArgFlags are the ssh options that take a value (see ssh(1)). Knowing
// them is what lets us tell `-p 22 host` (host is the destination) from
// `-N host`.
const sshArgFlags = "BbcDEeFIiJLlmOopQRSWw"

// SSHDestination returns the host an ssh command line connects to, ignoring
// options and any user@ prefix, e.g. `ssh -p 2222 me@box` gives "box". It is
// looser than SSHAlias on purpose: it only reports what a connection is
// (for the picker's indicator), never what to save, so capture still refuses
// to rewrite option-bearing commands into something different.
func SSHDestination(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || filepath.Base(fields[0]) != "ssh" {
		return "", false
	}
	for i := 1; i < len(fields); i++ {
		f := fields[i]
		if !strings.HasPrefix(f, "-") || f == "-" {
			return destHost(f)
		}
		// A cluster like -vp 22 ends at the first flag that takes a value;
		// the value is the rest of the word, or the next word if none.
		for j := 1; j < len(f); j++ {
			if strings.IndexByte(sshArgFlags, f[j]) >= 0 {
				if j == len(f)-1 {
					i++
				}
				break
			}
		}
	}
	return "", false
}

func destHost(dest string) (string, bool) {
	dest = strings.TrimPrefix(dest, "ssh://")
	if _, host, ok := strings.Cut(dest, "@"); ok {
		dest = host
	}
	if !workspace.ValidSSHAlias(dest) {
		return "", false
	}
	return dest, true
}

package workspace

import (
	"fmt"
	"regexp"
)

// RenameSSH points every pane that connects to the ssh alias from at the
// alias to instead, in every saved workspace that has one, and returns the
// names it changed. Files are edited in place: only the value of the `ssh`
// line changes, so comments, layout and quote style survive. A file that
// cannot be read, or whose rewrite does not load back as exactly the expected
// workspace, is left alone and named in the error. Each rewrite keeps the old
// file as name.toml + BackupSuffix and reaches the file by an atomic rename.
func RenameSSH(from, to string) ([]string, error) {
	if !aliasRe.MatchString(from) || !aliasRe.MatchString(to) {
		return nil, fmt.Errorf("invalid ssh alias %q or %q", from, to)
	}
	return eachWorkspace(func(path string) (bool, error) {
		return rewriteFile(path, from,
			func(lines []string) ([]string, error) { return renameLines(lines, from, to), nil },
			func(ws Workspace) Workspace { return expectedAfterRename(ws, from, to) })
	})
}

// expectedAfterRename is the workspace the rewritten file must load as.
func expectedAfterRename(ws Workspace, from, to string) Workspace {
	out := ws
	out.Windows = nil
	for _, w := range ws.Windows {
		nw := w
		nw.Panes = nil
		for _, p := range w.Panes {
			if p.SSH == from {
				p.SSH = to
			}
			nw.Panes = append(nw.Panes, p)
		}
		out.Windows = append(out.Windows, nw)
	}
	return out
}

// renameLines swaps the value of every `ssh = "from"` line inside a pane
// table. A trailing comment stays.
func renameLines(lines []string, from, to string) []string {
	out := append([]string{}, lines...)
	for _, q := range []string{`"`, `'`} {
		re := regexp.MustCompile(`^([ \t]*ssh[ \t]*=[ \t]*)` + q + regexp.QuoteMeta(from) + q + `([ \t]*(?:#.*)?)$`)
		for _, w := range scanWindows(out) {
			for _, p := range w.panes {
				for i := p.start + 1; i < p.end; i++ {
					if m := re.FindStringSubmatch(out[i]); m != nil {
						out[i] = m[1] + q + to + q + m[2]
					}
				}
			}
		}
	}
	return out
}

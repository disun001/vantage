package tui

// ruleW is the width of the hairline between two entries.
const ruleW = sidebarW - 2

// span lays out the sidebar list from entry top within capLines screen
// lines. flat marks the entries that stand alone (a session, a host, a
// project); a hairline goes above each of them except the first drawn. The
// result has one value per screen line: an entry index, or -1 for a hairline.
// Entries that hang off another (a window) never get a hairline above.
func span(flat []bool, top, capLines int) []int {
	var out []int
	for i := top; i < len(flat); i++ {
		need := 1
		if flat[i] && i > top {
			need = 2
		}
		if len(out)+need > capLines {
			break
		}
		if need == 2 {
			out = append(out, -1)
		}
		out = append(out, i)
	}
	return out
}

// scrollTop moves the first drawn entry so that the cursor entry pos is on
// screen, and so that no blank room is left below the last entry.
func scrollTop(flat []bool, top, pos, capLines int) int {
	top = max(0, min(top, len(flat)-1))
	top = min(top, pos)
	for top < pos && !drawn(span(flat, top, capLines), pos) {
		top++
	}
	for top > 0 {
		lines := span(flat, top-1, capLines)
		if len(lines) == 0 || lines[len(lines)-1] != len(flat)-1 {
			break
		}
		top--
	}
	return top
}

func drawn(lines []int, entry int) bool {
	for _, l := range lines {
		if l == entry {
			return true
		}
	}
	return false
}

// entryAt is the entry shown on sidebar screen line y, or -1 for a hairline
// or an empty line.
func entryAt(flat []bool, top, capLines, y int) int {
	lines := span(flat, top, capLines)
	if y < 0 || y >= len(lines) {
		return -1
	}
	return lines[y]
}

// allFlat marks n entries that all stand alone.
func allFlat(n int) []bool {
	f := make([]bool, n)
	for i := range f {
		f[i] = true
	}
	return f
}

// flatItems marks the session lines of the workspaces list; window lines
// hang off their session.
func (m model) flatItems(items []item) []bool {
	f := make([]bool, len(items))
	for i, it := range items {
		f[i] = it.Win < 0
	}
	return f
}

// ruleLine draws the hairline between two entries.
func ruleLine() string {
	l := lineBuf{}
	l.add(mocText, "", false, " ")
	l.add(mocSurf1, "", false, repeat("╌", ruleW))
	l.padTo(sidebarW)
	return l.String()
}

package tui

// span lays out the sidebar list from entry top within capLines screen
// lines. head marks the entries with a header line above them (the saved
// workspaces list has one); the header always draws with its entry. The
// result has one value per screen line: an entry index, or -1 for a header.
func span(head []bool, top, capLines int) []int {
	var out []int
	for i := top; i < len(head); i++ {
		need := 1
		if head[i] {
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
func scrollTop(head []bool, top, pos, capLines int) int {
	top = max(0, min(top, len(head)-1))
	top = min(top, pos)
	for top < pos && !drawn(span(head, top, capLines), pos) {
		top++
	}
	for top > 0 {
		lines := span(head, top-1, capLines)
		if len(lines) == 0 || lines[len(lines)-1] != len(head)-1 {
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

// entryAt is the entry shown on sidebar screen line y, or -1 for a header or
// an empty line.
func entryAt(head []bool, top, capLines, y int) int {
	lines := span(head, top, capLines)
	if y < 0 || y >= len(lines) {
		return -1
	}
	return lines[y]
}

// noHeads is the layout of n entries with no header line, as in the hosts
// and projects lists.
func noHeads(n int) []bool { return make([]bool, n) }

// headItems marks where the saved header goes: above the first workspace
// that is not running. Rows are ordered running first, saved-only after.
func (m model) headItems(items []item) []bool {
	h := make([]bool, len(items))
	for i, it := range items {
		if !m.rows[it.Row].Live {
			h[i] = true
			break
		}
	}
	return h
}

// savedHeader labels the workspaces that are saved but not running.
func savedHeader() string {
	l := lineBuf{}
	l.add(mocOverlay, "", false, " saved")
	l.padTo(sidebarW)
	return l.String()
}

package capture

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compactLayout turns tmux 3.8's JSON window layout into the classic layout
// string (`csum,WxH,x,y{...}`). The classic form is several times shorter,
// carries no pane ids that mean nothing after a restore, and is accepted by
// every tmux version, so a captured file stays portable.
//
// Layouts that are not JSON (older tmux already reports classic) pass
// through unchanged. A window with one pane needs no layout, so "" is
// returned for it.
func compactLayout(raw string) string {
	if !strings.HasPrefix(raw, "{") {
		return raw
	}
	var doc struct {
		L layoutNode `json:"L"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil || doc.L.T == "" {
		return raw // unknown shape: keep what tmux gave us rather than lose it
	}
	if doc.L.T == "p" {
		return ""
	}
	n := 0
	body := classic(doc.L, &n)
	return fmt.Sprintf("%04x,%s", checksum(body), body)
}

type layoutNode struct {
	T string       `json:"t"` // "p" pane, "h" side by side, "v" stacked
	W int          `json:"w"`
	H int          `json:"h"`
	X int          `json:"x"`
	Y int          `json:"y"`
	C []layoutNode `json:"c"`
}

// classic renders one node. Panes get sequential ids; tmux ignores them when
// applying a layout and only checks the pane count.
func classic(n layoutNode, next *int) string {
	s := fmt.Sprintf("%dx%d,%d,%d", n.W, n.H, n.X, n.Y)
	if n.T == "p" {
		id := *next
		*next++
		return fmt.Sprintf("%s,%d", s, id)
	}
	open, closer := "{", "}"
	if n.T == "v" {
		open, closer = "[", "]"
	}
	parts := make([]string, len(n.C))
	for i, c := range n.C {
		parts[i] = classic(c, next)
	}
	return s + open + strings.Join(parts, ",") + closer
}

// checksum is tmux's layout checksum (layout_checksum in layout-custom.c).
func checksum(s string) uint16 {
	var c uint16
	for i := 0; i < len(s); i++ {
		c = (c >> 1) + ((c & 1) << 15)
		c += uint16(s[i])
	}
	return c
}

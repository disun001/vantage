package workspace

import (
	"fmt"
	"strings"
	"unicode"
)

// MaxPaneTitle is the longest custom pane name, in characters. A pane border
// is one line, so a longer label would be cut off anyway.
const MaxPaneTitle = 40

// ValidPaneTitle checks a custom pane name. Empty is valid: it means "follow
// the running command". Control characters are refused because they can break
// the terminal, and '#' because tmux treats it as the start of a format or
// shell expansion (#(cmd)) wherever a name ends up inside a format string.
func ValidPaneTitle(s string) error {
	if n := len([]rune(s)); n > MaxPaneTitle {
		return fmt.Errorf("pane title is %d characters, the limit is %d", n, MaxPaneTitle)
	}
	if strings.ContainsRune(s, '#') {
		return fmt.Errorf("pane title %q cannot contain '#'", s)
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return fmt.Errorf("pane title cannot contain control characters")
	}
	return nil
}

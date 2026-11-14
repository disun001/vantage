package workspace

import (
	"strings"
	"testing"
)

func TestValidPaneTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		ok    bool
	}{
		{"empty clears", "", true},
		{"plain", "api server", true},
		{"unicode", "ビルド ✓", true},
		{"max length", strings.Repeat("a", MaxPaneTitle), true},
		{"too long", strings.Repeat("a", MaxPaneTitle+1), false},
		{"format expansion", "#{pane_id}", false},
		{"shell expansion", "#(rm -rf ~)", false},
		{"newline", "a\nb", false},
		{"escape", "a\x1b[31m", false},
		{"tab", "a\tb", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidPaneTitle(tt.title); (err == nil) != tt.ok {
				t.Errorf("ValidPaneTitle(%q) = %v, want ok=%v", tt.title, err, tt.ok)
			}
		})
	}
}

func TestValidateRejectsBadPaneTitle(t *testing.T) {
	ws := Workspace{Name: "w", Windows: []Window{{Panes: []Pane{{Title: "#(x)"}}}}}
	if err := ws.Validate(); err == nil {
		t.Fatal("want error for a title with '#'")
	}
	ws.Windows[0].Panes[0].Title = "ok"
	if err := ws.Validate(); err != nil {
		t.Fatal(err)
	}
}

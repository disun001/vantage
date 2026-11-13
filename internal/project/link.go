package project

import (
	"fmt"

	"github.com/disun001/vantage/internal/workspace"
)

// ApplyRoot fills an empty root of a linked workspace with the project dir. A
// workspace with an explicit root, or with no project, is left alone.
func ApplyRoot(ws *workspace.Workspace) error {
	if ws.Project == "" || ws.Root != "" {
		return nil
	}
	p, err := Load(ws.Project)
	if err != nil {
		return fmt.Errorf("workspace %q: %w", ws.Name, err)
	}
	root, err := Root()
	if err != nil {
		return fmt.Errorf("workspace %q: %w", ws.Name, err)
	}
	ws.Root = p.Path(root)
	return nil
}

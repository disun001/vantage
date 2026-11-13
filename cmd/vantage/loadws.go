package main

import (
	"github.com/disun001/vantage/internal/project"
	"github.com/disun001/vantage/internal/workspace"
)

// loadWorkspace loads a workspace file and fills the root of a project link,
// so every command that builds a session sees the same directory.
func loadWorkspace(path string) (workspace.Workspace, error) {
	ws, err := workspace.Load(path)
	if err != nil {
		return ws, err
	}
	return ws, project.ApplyRoot(&ws)
}

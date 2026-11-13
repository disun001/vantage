package project

import (
	"fmt"
	"io"
	"os"

	"github.com/disun001/vantage/internal/workspace"
)

// ApplyRoot fills an empty root of a linked workspace: the task dir for a
// workspace with a task, else the project dir. A workspace with an explicit
// root, or with no project, is left alone.
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
	if ws.Task != "" {
		ws.Root = p.TaskPath(root, ws.Task)
	}
	return nil
}

// FindTaskWorkspace returns the name of the workspace linked to the task by
// its project and task keys, not by file name, so it may be renamed. Files
// that cannot be read are skipped.
func FindTaskWorkspace(project, task string) (string, bool, error) {
	names, err := workspace.List()
	if err != nil {
		return "", false, err
	}
	for _, n := range names {
		path, err := workspace.PathOf(n)
		if err != nil {
			continue
		}
		if ws, err := workspace.Load(path); err == nil && ws.Project == project && ws.Task == task {
			return ws.Name, true, nil
		}
	}
	return "", false, nil
}

// EnsureTaskWorkspace returns the workspace of a task, creating it when none
// is linked: one window and one pane at the task root, named <project>-<task>.
// The pane runs nothing; the user adds `claude` or anything else. The root is
// not written, it follows from the link (see ApplyRoot).
func EnsureTaskWorkspace(root string, p Project, task string) (name string, created bool, err error) {
	if err := ValidTaskName(task); err != nil {
		return "", false, err
	}
	if _, err := os.Stat(p.TaskPath(root, task)); err != nil {
		return "", false, fmt.Errorf("task %q not found", task)
	}
	if name, ok, err := FindTaskWorkspace(p.Name, task); err != nil || ok {
		return name, false, err
	}
	name = p.Name + "-" + task
	path, err := workspace.PathOf(name)
	if err != nil {
		return "", false, err
	}
	ws := workspace.Workspace{
		Name: name, Project: p.Name, Task: task,
		Windows: []workspace.Window{{Name: task, Panes: []workspace.Pane{{}}}},
	}
	if err := workspace.Save(path, ws, false); err != nil {
		return "", false, fmt.Errorf("create workspace %q: %w", name, err)
	}
	return name, true, nil
}

// warnWorkspace tells the user that a removed task's workspace file is still
// there. It is never deleted automatically: it may hold edits.
func warnWorkspace(project, task string, out io.Writer) {
	name, ok, err := FindTaskWorkspace(project, task)
	if err != nil || !ok {
		return
	}
	fmt.Fprintf(out, "warn    workspace %q still points at removed task %q; delete it when you are done\n", name, task)
}

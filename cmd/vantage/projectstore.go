package main

import (
	"fmt"
	"io"

	"github.com/disun001/vantage/internal/git"
	"github.com/disun001/vantage/internal/project"
	"github.com/disun001/vantage/internal/tui"
)

// projectStore adapts internal/project to the TUI's projects view.
type projectStore struct{ git git.Client }

func (projectStore) List() ([]string, error) { return project.List() }

func (s projectStore) Detail(name string) (tui.ProjectInfo, error) {
	p, root, err := loadProject(name)
	if err != nil {
		return tui.ProjectInfo{}, err
	}
	info := tui.ProjectInfo{Name: p.Name, Flat: p.Flat}
	for _, r := range project.Check(root, p, s.git) {
		info.Repos = append(info.Repos, repoInfo(r))
	}
	tasks, err := project.ListTasks(root, p)
	if err != nil {
		return info, err
	}
	for _, t := range tasks {
		ti := tui.TaskInfo{Name: t}
		for _, r := range project.TaskRepos(root, p, t) {
			ti.Repos = append(ti.Repos, r.DirName())
		}
		info.Tasks = append(info.Tasks, ti)
	}
	return info, nil
}

func repoInfo(r project.RepoStatus) tui.RepoInfo {
	ri := tui.RepoInfo{Name: r.Repo.DirName(), Cloned: r.Cloned}
	if r.Err != nil {
		ri.Err = r.Err.Error()
		return ri
	}
	ri.Branch, ri.Sync, ri.Dirty = branchLabel(r), syncLabel(r), r.Git.Dirty
	ri.Warn = joinWarnings(r.Warnings)
	return ri
}

// Create saves the definition, then clones the repos. A clone failure leaves
// the project defined, so `vantage project sync` can finish it.
func (s projectStore) Create(d tui.ProjectDef) error {
	p := project.Project{Name: d.Name, Flat: d.Flat}
	for _, r := range d.Repos {
		p.Repos = append(p.Repos, project.Repo{URL: r.URL, Dir: r.Dir, Copy: r.Copy})
	}
	if err := project.Save(p); err != nil {
		return err
	}
	root, err := project.Root()
	if err != nil {
		return fmt.Errorf("project saved, but %w", err)
	}
	if err := project.Sync(root, p, s.git, io.Discard); err != nil {
		return fmt.Errorf("project saved, but cloning failed (run: vantage project sync %s): %w", d.Name, err)
	}
	return nil
}

func (s projectStore) NewTask(name, task string, picks []tui.TaskPick) error {
	p, root, err := loadProject(name)
	if err != nil {
		return err
	}
	repos := make([]project.TaskRepo, len(picks))
	for i, pk := range picks {
		repos[i] = project.TaskRepo{Dir: pk.Repo, Base: pk.Base}
	}
	return project.NewTask(root, p, s.git, task, repos, io.Discard)
}

func (s projectStore) Remove(name string, force bool) error {
	p, root, err := loadProject(name)
	if err != nil {
		return err
	}
	return project.Remove(root, p, s.git, force, io.Discard)
}

func (s projectStore) RemoveTask(name, task string, force bool) error {
	p, root, err := loadProject(name)
	if err != nil {
		return err
	}
	return project.RemoveTask(root, p, s.git, task, force, io.Discard)
}

func (projectStore) Workspace(name, task string) (string, error) {
	p, root, err := loadProject(name)
	if err != nil {
		return "", err
	}
	ws, _, err := project.EnsureTaskWorkspace(root, p, task)
	return ws, err
}

func loadProject(name string) (project.Project, string, error) {
	p, err := project.Load(name)
	if err != nil {
		return p, "", err
	}
	root, err := project.Root()
	return p, root, err
}

package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/disun001/vantage/internal/git"
	"github.com/disun001/vantage/internal/project"
)

var taskCmd = command{
	name:    "task",
	usage:   "task list|new|rm|workspace <project> ...",
	summary: "manage tasks (git worktrees across a project's repos)",
	run:     runTask,
}

const taskUsage = "usage: vantage task list <project> | new <project> <task> [--repo name[:base]]... | rm <project> <task> [--force] | workspace <project> <task>"

// taskArgs is a parsed `task` command line. Flags may appear anywhere.
type taskArgs struct {
	pos   []string
	repos []project.TaskRepo
	force bool
}

func parseTaskArgs(args []string) (taskArgs, error) {
	var t taskArgs
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--force":
			t.force = true
		case a == "--repo" || strings.HasPrefix(a, "--repo="):
			v := strings.TrimPrefix(a, "--repo=")
			if a == "--repo" {
				if i++; i >= len(args) {
					return t, errors.New("--repo needs a value")
				}
				v = args[i]
			}
			tr, err := parseRepoFlag(v)
			if err != nil {
				return t, err
			}
			t.repos = append(t.repos, tr)
		case strings.HasPrefix(a, "-"):
			return t, fmt.Errorf("unknown flag %q", a)
		default:
			t.pos = append(t.pos, a)
		}
	}
	return t, nil
}

// parseRepoFlag parses name[:base]. A branch name cannot contain ':'.
func parseRepoFlag(v string) (project.TaskRepo, error) {
	name, base, hasBase := strings.Cut(v, ":")
	if name == "" || (hasBase && base == "") {
		return project.TaskRepo{}, fmt.Errorf("invalid --repo %q (want name or name:base)", v)
	}
	return project.TaskRepo{Dir: name, Base: base}, nil
}

func runTask(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(taskUsage)
	}
	sub := args[0]
	t, err := parseTaskArgs(args[1:])
	if err != nil {
		return err
	}
	want := map[string]int{"list": 1, "new": 2, "rm": 2, "workspace": 2}[sub]
	if want == 0 {
		return fmt.Errorf("unknown task command %q", sub)
	}
	if len(t.pos) != want || (sub != "new" && len(t.repos) > 0) || (sub != "rm" && t.force) {
		return errors.New(taskUsage)
	}
	p, err := project.Load(t.pos[0])
	if err != nil {
		return err
	}
	root, err := project.Root()
	if err != nil {
		return err
	}
	switch sub {
	case "list":
		return taskList(root, p, out)
	case "new":
		return project.NewTask(root, p, git.Client{}, t.pos[1], t.repos, out)
	case "workspace":
		return taskWorkspace(root, p, t.pos[1], out)
	}
	return project.RemoveTask(root, p, git.Client{}, t.pos[1], t.force, out)
}

// taskWorkspace prints the name of the task's workspace, creating it if none
// is linked.
func taskWorkspace(root string, p project.Project, task string, out io.Writer) error {
	name, created, err := project.EnsureTaskWorkspace(root, p, task)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(out, "created workspace %s\n", name)
		return nil
	}
	fmt.Fprintf(out, "workspace %s\n", name)
	return nil
}

func taskList(root string, p project.Project, out io.Writer) error {
	names, err := project.ListTasks(root, p)
	if err != nil {
		return err
	}
	for _, n := range names {
		var repos []string
		for _, r := range project.TaskRepos(root, p, n) {
			repos = append(repos, r.DirName())
		}
		fmt.Fprintf(out, "%s\t%s\n", n, strings.Join(repos, ","))
	}
	return nil
}

package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/disun001/vantage/internal/git"
	"github.com/disun001/vantage/internal/project"
)

var projectCmd = command{
	name:    "project",
	usage:   "project list|sync|status|update|rm [name]",
	summary: "manage projects (sets of git repos)",
	run:     runProject,
}

func runProject(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: vantage project list|sync|status|update|rm [name]")
	}
	sub, rest := args[0], args[1:]
	if sub == "rm" {
		return runProjectRm(rest, out)
	}
	if sub == "list" {
		return projectList(out)
	}
	if sub != "sync" && sub != "status" && sub != "update" {
		return fmt.Errorf("unknown project command %q", sub)
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: vantage project %s <name>", sub)
	}
	p, err := project.Load(rest[0])
	if err != nil {
		return err
	}
	root, err := project.Root()
	if err != nil {
		return err
	}
	switch sub {
	case "sync":
		return project.Sync(root, p, git.Client{}, out)
	case "update":
		return project.Update(root, p, git.Client{}, out)
	}
	return printStatus(out, project.Check(root, p, git.Client{}))
}

// runProjectRm handles `project rm <name> [--force]`.
func runProjectRm(args []string, out io.Writer) error {
	const usage = "usage: vantage project rm <name> [--force]"
	force, names := false, []string{}
	for _, a := range args {
		switch {
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			names = append(names, a)
		}
	}
	if len(names) != 1 {
		return errors.New(usage)
	}
	p, root, err := loadProject(names[0])
	if err != nil {
		return err
	}
	return project.Remove(root, p, git.Client{}, force, out)
}

func projectList(out io.Writer) error {
	names, err := project.List()
	if err != nil {
		return err
	}
	for _, n := range names {
		fmt.Fprintln(out, n)
	}
	return nil
}

// printStatus renders one row per repo. A repo whose status cannot be read is
// shown and reported in the returned error, so the exit code is non-zero.
func printStatus(out io.Writer, rows []project.RepoStatus) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REPO\tBRANCH\tSYNC\tDIRTY\tWARN")
	var errs []error
	for _, r := range rows {
		name := r.Repo.DirName()
		switch {
		case r.Err != nil:
			fmt.Fprintf(tw, "%s\terror\t\t\t%v\n", name, r.Err)
			errs = append(errs, fmt.Errorf("%s: %w", name, r.Err))
		case !r.Cloned:
			fmt.Fprintf(tw, "%s\tnot cloned\t\t\trun: vantage project sync\n", name)
		default:
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", name, branchLabel(r), syncLabel(r),
				yesNo(r.Git.Dirty), joinWarnings(r.Warnings))
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	return errors.Join(errs...)
}

func branchLabel(r project.RepoStatus) string {
	if r.Git.Branch == "" {
		return "(detached)"
	}
	return r.Git.Branch
}

func syncLabel(r project.RepoStatus) string {
	switch {
	case !r.Git.HasUpstream:
		return "no upstream"
	case r.Git.Ahead == 0 && r.Git.Behind == 0:
		return "up to date"
	default:
		return fmt.Sprintf("+%d -%d", r.Git.Ahead, r.Git.Behind)
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func joinWarnings(w []string) string {
	s := ""
	for i, x := range w {
		if i > 0 {
			s += "; "
		}
		s += x
	}
	return s
}

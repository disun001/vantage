# Projects (design)

Status: designed. Steps 1 (projects path, project file, `sync`, `status`) and 2 (`update`) are built; the rest is not. Build order at the end.

A **project** is a set of git repos managed together. A **task** is a unit of work that spans some of those repos, isolated in git worktrees. Vantage keeps the repos fresh and creates task worktrees; the user (and later, agents) work inside tasks.

## Layout on disk

```
<projects>/<name>/<repo>/          main clones: pristine bases
<projects>/<name>.tasks/<task>/<repo>/   one worktree per included repo
```

- The project dir is a plain directory, not a git repo. Claude runs from a task dir and sees every repo of that task.
- `<projects>` is `$VANTAGE_PROJECTS_DIR`, else `projects_dir` in `<config>/config.toml` (`~` expands). No default. When unset, `project` commands fail with a clear error.
- The project definition lives in `<config>/projects/<name>.toml`, next to `workspaces/`. The project dir holds only clones.

## Project file

```toml
name = "api-stack"            # same charset rules as workspace names

[[repo]]
url = "git@example.com:org/api.git"
dir = "api"                   # optional, defaults to the repo name
copy = [".env", ".env.local"] # untracked files copied into task worktrees

[[repo]]
url = "git@example.com:org/web.git"
```

Unknown keys are an error. `url`, `dir`, `copy` entries and branch names are validated before they reach git (AGENTS.md rule 3). Vantage runs plain `git clone`; the user's own git auth applies, and vantage never reads `~/.ssh` (rule 2). Creating a project only clones; existing checkouts are not adopted. For now a project is created by writing the file by hand (see `examples/projects/`); a TUI form comes with step 5. `vantage project list` shows defined projects.

## Main clones

Pristine bases: the user and agents work in task worktrees, not here.

- `project sync`: clone repos that are missing.
- `project status`: per repo, branch, ahead/behind, dirty, worktree count. Warns when a main clone is dirty or off its default branch. Never blocks or resets.
- `project update`: `git fetch` everywhere, then fast-forward (`merge --ff-only`) only clones that are clean, on their default branch, with an upstream and not diverged. Others are reported as skipped.

## Tasks

State is derived from disk and `git worktree list`. No database yet; task metadata (status, agent) joins the SQLite store when orchestration is designed.

- `task new <project> <task> [--repo name[:base]]...`
  - No `--repo`: a prompt form with the task name and one row per repo (include checkbox, base branch defaulting to that repo's default branch).
  - With `--repo`: no prompt. The prompt and the flags build the same request.
  - Each included repo gets a worktree on branch `<task>`, cut from its fetched base. The repo's `copy` files are copied from its main clone. File contents are never printed or logged. Warn if a `copy` path is tracked by git.
- `task rm <project> <task> [--force]`
  - All or nothing: refuses if any repo has uncommitted, unpushed or unmerged work. `--force` overrides the check.
  - Removes the worktrees, then the task dir. Deletes a branch only when fully merged into its base (`git branch -d`, never `-D`), even with `--force`.
  - Warns about the task's workspace file.

## Workspace link

Workspaces gain two optional keys:

| Key       | Meaning                                                                 |
| --------- | ----------------------------------------------------------------------- |
| `project` | Project name. `root` defaults to the project dir. Validated.            |
| `task`    | Needs `project`. `root` defaults to the task dir. Validated.            |

A task's workspace is found by the `project` and `task` keys, not by file name, so it can be renamed. When none exists, vantage creates one: a single window, one pane, `cwd` = task root, named `<project>-<task>`. The pane does not run `claude`; the user adds that.

## TUI

- A projects view in the sidebar, swapped in like the hosts view (`H`). Key to be chosen.
- With a project selected, the right pane shows the project details instead of the embedded tmux: repo status, task list, add task (the same form as `task new`).
- Selecting or creating a task returns to the workspaces sidebar on that task's workspace: opened if it exists, created if not. If its session is live, focus it.

## Deferred

- An opt-in project setting that adds a `claude` pane to new task workspaces (`autorun` rules apply, rule 3).
- Orchestration and task metadata in SQLite.
- Adopting or copying existing local clones.
- A default session for a project with no workspace.

## Build order

1. Projects path, project TOML, `sync`, `status`. **Done.**
2. `update`. **Done.**
3. Tasks: `new`, `rm`, `copy` files.
4. Workspace `project` and `task` keys.
5. TUI projects view and details page.

Each step updates its tests, this file, `docs/workspace-format.md` (step 4) and `examples/`.

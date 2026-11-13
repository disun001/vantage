# Projects (design)

Status: built. Steps 1 (projects path, project file, `sync`, `status`), All five build steps are done. Build order at the end.

A **project** is a set of git repos managed together. A **task** is a unit of work that spans some of those repos, isolated in git worktrees. Vantage keeps the repos fresh and creates task worktrees; the user (and later, agents) work inside tasks.

## Layout on disk

```
<projects>/<name>/<repo>/          main clones: pristine bases
<projects>/.tasks/<name>/<task>/<repo>/  one worktree per included repo
```

- Flat: a project with `flat = true` has exactly one repo and no `dir`. The clone is `<projects>/<name>/` itself and each task worktree is `<projects>/.tasks/<name>/<task>/` itself, so Claude starts inside the repo and loads its `AGENTS.md`. Not a default: adding a second repo later means removing `flat` and moving the clone.
- All task worktrees live under one hidden dir, `<projects>/.tasks/`, so the projects dir lists only projects. Project names start with a letter or digit, so none can collide with `.tasks`.
- The project dir is a plain directory, not a git repo. Claude runs from a task dir and sees every repo of that task.
- `<projects>` is `$VANTAGE_PROJECTS_DIR`, else `projects_dir` in `<config>/config.toml` (`~` expands). No default. When unset, `project` commands fail with a clear error.
- The project definition lives in `<config>/projects/<name>.toml`, next to `workspaces/`. The project dir holds only clones.

## Project file

```toml
name = "api-stack"            # same charset rules as workspace names
# flat = true                 # one repo only: clone at <projects>/<name>, no nesting

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
- `project rm <name> [--force]`: remove every task, the main clones and the project file. Refuses if a task or clone has uncommitted or unpushed work (tasks: also unmerged); `--force` skips that. Workspace files are kept.
- `project status`: per repo, branch, ahead/behind, dirty, worktree count. Warns when a main clone is dirty or off its default branch. Never blocks or resets.
- `project update`: `git fetch` everywhere, then fast-forward (`merge --ff-only`) only clones that are clean, on their default branch, with an upstream and not diverged. Others are reported as skipped.

## Tasks

State is derived from disk and `git worktree list`. No database; task metadata (status, agent) would need a store, to be designed with orchestration.

- `task list <project>`: task names and their repos.
- `task new <project> <task> [--repo name[:base]]...`
  - The CLI is non-interactive: no `--repo` means every repo at its default branch. The prompt form (task name, one row per repo with an include checkbox and base branch) comes with the TUI in step 5; it builds the same request as the flags.
  - Each included repo gets a worktree on branch `<task>`, cut from its fetched base (`origin/<base>`, else a local branch; the branch does not track it). All or nothing: every check runs first, and a failure part way removes the worktrees already made. Then the repo's `copy` files are copied from its main clone: regular files only, no symlinks or anything under `.git`, never overwriting, contents never printed. A tracked `copy` path is warned about and not copied. A copy failure keeps the task and exits non-zero.
- `task rm <project> <task> [--force]`
  - All or nothing: refuses if any repo has uncommitted, unpushed or unmerged work. `--force` overrides the check.
  - Removes the worktrees, then the task dir. Deletes a branch only when fully merged (`git branch -d`, never `-D`, even with `--force`). It judges against the main clone's HEAD, so run `project update` first; an unmerged branch is kept and reported.
  - Warns if a workspace is still linked to the task. The file is never deleted automatically.

## Workspace link

Workspaces gain two optional keys:

| Key       | Meaning                                                                 |
| --------- | ----------------------------------------------------------------------- |
| `project` | Project name. `root` defaults to the project dir. Validated.            |
| `task`    | Needs `project`. `root` defaults to the task dir. Validated.            |

A task's workspace is found by the `project` and `task` keys, not by file name, so it can be renamed. `vantage task workspace <project> <task>` returns it, creating it when none exists (the TUI does the same in step 5): a single window, one pane, `cwd` = task root, named `<project>-<task>`. The pane does not run `claude`; the user adds that.

## TUI

`P` in the workspaces sidebar swaps it for the projects list (like `H` for hosts). The right pane shows the details of the project under the cursor instead of tmux: each repo's branch, sync state, dirty flag and warnings, then the task list. Details load in the background; the TUI never blocks on git.

| Key (projects list) | Does                                                              |
| ------------------- | ----------------------------------------------------------------- |
| `j` `k` `g` `G`     | Move; the details follow                                          |
| `⏎` `l` `tab`       | Step into the task list (when there are tasks)                    |
| `n`                 | New task form                                                     |
| `a`                 | Add project form                                                  |
| `r`                 | Refresh                                                           |
| `P` `esc` `h`       | Back to workspaces                                                |

| Key (task list) | Does                                    |
| --------------- | --------------------------------------- |
| `j` `k` `g` `G` | Move                                    |
| `⏎`             | Open the task's workspace               |
| `esc` `h`       | Back to the projects list               |

- **New task form:** the task name, then one line per repo with `[x]` include (`space` toggles) and a base branch (type it; empty means the repo's default). `enter` creates the task, then opens its workspace.
- **Add project form:** name, a `flat` toggle (`space`), and per repo `url`, `dir` and `copy` (comma-separated globs). `ctrl+a` adds a repo, `ctrl+x` drops the focused one. `enter` writes the project file and clones the repos. If cloning fails the project stays defined; finish with `vantage project sync`.
- **Opening a task** (from the list, or right after `new`) returns to the workspaces sidebar on that task's workspace: found by its `project` and `task` keys, created when none exists, its session started if it is not live, then shown with focus.
- A failure (a taken branch, a bad URL) stays on the details page as a notice.
- `alt+s` and mouse clicks on the details page do nothing: there is no tmux to focus.
- **Delete:** `d` on the project list deletes the project, `d` on the task list deletes the task. `y` is the safe delete (refuses if work would be lost, the reason shows in the notice), `f` forces.
- Not in the TUI: `update`, `sync` and editing a project. Use the CLI.

## Deferred

- An opt-in project setting that adds a `claude` pane to new task workspaces (`autorun` rules apply, rule 3).
- Orchestration and task metadata (needs a store).
- Adopting or copying existing local clones.
- A default session for a project with no workspace.

## Build order

1. Projects path, project TOML, `sync`, `status`. **Done.**
2. `update`. **Done.**
3. Tasks: `new`, `rm`, `copy` files. **Done.**
4. Workspace `project` and `task` keys. **Done.**
5. TUI projects view and details page. **Done.**

Each step updates its tests, this file, `docs/workspace-format.md` (step 4) and `examples/`.

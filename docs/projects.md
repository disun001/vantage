# Projects

A **project** is a set of git repos managed together. Vantage clones them, keeps them fresh and shows their state. Nothing else: there is no task, worktree or orchestration layer.

## Layout on disk

```
<projects>/<name>/<repo>/   one clone per repo
```

- Flat: a project with `flat = true` has exactly one repo and no `dir`. The clone is `<projects>/<name>/` itself. Adding a second repo later means removing `flat` and moving the clone.
- The project dir is a plain directory, not a git repo.
- `<projects>` is `$VANTAGE_PROJECTS_DIR`, else `projects_dir` in `<config>/config.toml` (`~` expands). No default. When unset, `project` commands fail with a clear error.
- The project definition lives in `<config>/projects/<name>.toml`, next to `workspaces/`. The project dir holds only clones.

## Project file

```toml
name = "api-stack"            # same charset rules as workspace names
# flat = true                 # one repo only: clone at <projects>/<name>, no nesting

[[repo]]
url = "git@example.com:org/api.git"
dir = "api"                   # optional, defaults to the repo name

[[repo]]
url = "git@example.com:org/web.git"
```

Unknown keys are an error. `url` and `dir` are validated before they reach git (AGENTS.md rule 3). Vantage runs plain `git clone`; the user's own git auth applies, and vantage never reads `~/.ssh` (rule 2). Creating a project only clones; existing checkouts are not adopted. Create a project from the TUI (`a`) or by writing the file by hand (see `examples/projects/`). `vantage project list` shows defined projects.

## Commands

- `project sync`: clone repos that are missing.
- `project rm <name> [--force]`: remove the clones and the project file. Refuses if a clone has uncommitted or unpushed work; `--force` skips that. Workspace files are kept.
- `project status`: per repo, branch, ahead/behind, dirty. Warns when a clone is dirty or off its default branch. Never blocks or resets.
- `project update`: `git fetch` everywhere, then fast-forward (`merge --ff-only`) only clones that are clean, on their default branch, with an upstream and not diverged. Others are reported as skipped.

## Workspace link

Workspaces gain one optional key:

| Key       | Meaning                                                      |
| --------- | ------------------------------------------------------------ |
| `project` | Project name. `root` defaults to the project dir. Validated. |

## TUI

`P` in the workspaces sidebar swaps it for the projects list (like `H` for hosts). The right pane shows the details of the project under the cursor instead of tmux: each repo's branch, sync state, dirty flag and warnings. Details load in the background; the TUI never blocks on git.

| Key (projects list) | Does                                 |
| ------------------- | ------------------------------------ |
| `j` `k` `g` `G`     | Move; the details follow             |
| `a`                 | Add project form                     |
| `d`                 | Delete the project                   |
| `r`                 | Refresh                              |
| `P` `esc` `h`       | Back to workspaces                   |

- **Add project form:** name, a `flat` toggle (`space`), and per repo `url` and `dir`. `ctrl+a` adds a repo, `ctrl+x` drops the focused one. `enter` writes the project file and clones the repos. If cloning fails the project stays defined; finish with `vantage project sync`.
- A failure (a bad URL, a refused delete) stays on the details page as a notice.
- `alt+s` and mouse clicks on the details page do nothing: there is no tmux to focus.
- **Delete:** `y` is the safe delete (refuses if work would be lost, the reason shows in the notice), `f` forces.
- Not in the TUI: `update`, `sync` and editing a project. Use the CLI.

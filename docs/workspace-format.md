# Workspace format

One TOML file per workspace, in `~/.config/vantage/workspaces/<name>.toml`
(see [installation](installation.md) for overrides). Unknown keys are an error,
so typos surface immediately.

```toml
name = "api"                 # tmux session name; defaults to the file name
root = "~/Work/api"          # default cwd; "~" expands

[[window]]
name = "edit"
layout = "main-vertical"     # tmux preset or a full layout string

  [[window.pane]]
  cwd = "."                  # relative to root
  cmd = "nvim"
  autorun = false

  [[window.pane]]
  split = "v"

[[window]]
name = "db"

  [[window.pane]]
  ssh = "prod-db"            # alias from ~/.ssh/config
```

## Fields

| Key               | Level     | Meaning                               | Rules                                                                          |
| ----------------- | --------- | ------------------------------------- | ------------------------------------------------------------------------------ |
| `name`            | workspace | Session name                          | `[A-Za-z0-9][A-Za-z0-9_-]*`. No `.`, `:`, spaces (tmux rewrites them)          |
| `root`            | workspace | Default pane directory                | Optional. `~` expands                                                          |
| `project`         | workspace | Link to a project (`vantage project`) | Optional. Same charset as `name`. When `root` is empty, `root` is the project dir          |
| `task`            | workspace | Link to one of its tasks              | Optional. Needs `project`. When `root` is empty, `root` is the task dir                    |
| `[[window]]`      | workspace | Windows, in order                     | At least one                                                                   |
| `name`            | window    | Window name                           | Optional                                                                       |
| `layout`          | window    | `select-layout` argument              | Optional. Applied after all panes exist                                        |
| `[[window.pane]]` | window    | Panes, in order                       | At least one                                                                   |
| `cwd`             | pane      | Directory                             | Relative paths join `root`. Empty means `root`                                 |
| `ssh`             | pane      | ssh config alias                      | Matches `[A-Za-z0-9][A-Za-z0-9._@%+-]*`. Cannot start with `-`. Not with `cmd` |
| `cmd`             | pane      | Command to type into the pane's shell | Not with `ssh`                                                                 |
| `autorun`         | pane      | Allow `cmd` to run on restore         | Default false. Needs `cmd`                                                     |
| `chat`            | pane      | Chat id; restore reopens that chat    | 16 lowercase hex digits (`vantage chat list`). Not with `ssh` or `cmd`         |
| `split`           | pane      | How it splits from the previous pane  | `h` (side by side, default) or `v` (stacked). Ignored on the first pane        |

## Project and task link

`project` and `task` tie a workspace to a task of a project (see [projects](projects.md)). A task's workspace is found by these keys, not by file name, so the file can be renamed. `restore` and the TUI fill an empty `root` from the link (`project.ApplyRoot`); an explicit `root` wins. Saving over a file (capture `--force`, `S`) keeps the link, since a live session knows nothing of projects. `vantage task workspace <project> <task>` creates the workspace of a task when none is linked: one window, one bare pane, named `<project>-<task>`.

## Restore semantics

- If a session with the same name is running, `restore` does nothing.
- If creation fails part way, the half-built session is removed.
- SSH panes open a normal shell, then type `ssh <alias>`, so the pane survives a dropped connection.
- Commands without `autorun = true` are never run.
- A `chat` pane runs `vantage chat <id>` and resumes that conversation. If the id is unknown (for example the database was wiped), the pane shows the error and waits for Enter.

## Capture semantics

`vantage capture <session> [--name N] [--force] [--socket S]` writes `<config>/workspaces/<name>.toml` from a running session.

- Windows keep their name and layout. tmux 3.8 reports layouts as long JSON, so capture rewrites them to the short classic string (`csum,WxH,x,y{...}`) that every tmux version accepts. A one-pane window saves no layout.
- Panes keep their directory (home is written as `~`).
- A pane whose foreground process is exactly `ssh <alias>` becomes `ssh = "<alias>"`.
- Any other foreground command (including `ssh` with options or ports) is saved as `cmd` with `autorun = false`, so restore never runs it unless you edit the file.
- A pane running a chat (found through the chat database, matched by pane id on the same tmux server) becomes `chat = "<id>"`, never a `cmd`.
- Idle shells save nothing. An existing file is only replaced with `--force`.
- Foreground detection uses `ps -t <tty>`. If it fails, only the command name is saved.

## Save all (`S` in the sidebar)

`S` asks for `y`, then captures **every running session** and replaces its workspace file (creating it when missing). The old file is first copied to `<name>.toml.vantage.bak`, because a capture drops comments and `autorun = true` lines. A session that cannot be saved (invalid name) is reported and the rest still go. Saved workspaces that are not running are left alone.

## Changing the schema

Update `internal/workspace`, its tests, this file, and `examples/`. New fields
that reach a shell or tmux target must be validated (AGENTS.md rule 3).

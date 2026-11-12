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
| `[[window]]`      | workspace | Windows, in order                     | At least one                                                                   |
| `name`            | window    | Window name                           | Optional                                                                       |
| `layout`          | window    | `select-layout` argument              | Optional. Applied after all panes exist                                        |
| `[[window.pane]]` | window    | Panes, in order                       | At least one                                                                   |
| `cwd`             | pane      | Directory                             | Relative paths join `root`. Empty means `root`                                 |
| `ssh`             | pane      | ssh config alias                      | Matches `[A-Za-z0-9][A-Za-z0-9._@%+-]*`. Cannot start with `-`. Not with `cmd` |
| `cmd`             | pane      | Command to type into the pane's shell | Not with `ssh`                                                                 |
| `autorun`         | pane      | Allow `cmd` to run on restore         | Default false. Needs `cmd`                                                     |
| `split`           | pane      | How it splits from the previous pane  | `h` (side by side, default) or `v` (stacked). Ignored on the first pane        |

## Restore semantics

- If a session with the same name is running, `restore` does nothing.
- If creation fails part way, the half-built session is removed.
- SSH panes open a normal shell, then type `ssh <alias>`, so the pane survives a dropped connection.
- Commands without `autorun = true` are never run.

## Capture semantics

`vantage capture <session> [--name N] [--force] [--socket S]` writes `<config>/workspaces/<name>.toml` from a running session.

- Windows keep their name and tmux layout string; panes keep their directory (home is written as `~`).
- A pane whose foreground process is exactly `ssh <alias>` becomes `ssh = "<alias>"`.
- Any other foreground command (including `ssh` with options or ports) is saved as `cmd` with `autorun = false`, so restore never runs it unless you edit the file.
- Idle shells save nothing. An existing file is only replaced with `--force`.
- Foreground detection uses `ps -t <tty>`. If it fails, only the command name is saved.

## Changing the schema

Update `internal/workspace`, its tests, this file, and `examples/`. New fields
that reach a shell or tmux target must be validated (AGENTS.md rule 3).

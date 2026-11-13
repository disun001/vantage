# Layout

```text
cmd/vantage/        CLI: main.go dispatches; one file per subcommand
                    (list.go, restore.go, capture.go, pick.go, tui.go); flags.go holds shared flag helpers
internal/workspace/ workspace schema, load, validate, config dir lookup
internal/tmux/      thin adapter over the tmux CLI (the only place that execs tmux)
internal/git/       thin adapter over the git CLI (the only place that execs git)
internal/project/   project schema, load, validate, projects dir, clone and status of main clones
internal/restore/   builds a live session from a Workspace
internal/manage/    create, rename, delete workspaces (file and live session together)
internal/capture/   reads a live session back into a Workspace (ps lookup for ssh/commands)
internal/sshconfig/ list hosts (alias, address, user, port, jump) from ssh_config; guarded add and edit of Host blocks
internal/hostuse/  what depends on an ssh alias (workspaces, live panes) before it is deleted; closes the panes, strips saved ones
internal/connections/ which ssh hosts are already open in live tmux panes
internal/pick/      fuzzy menu (local or ssh) and the split it creates
internal/embed/     runs a program in a PTY and mirrors it in a virtual screen
internal/tui/       sidebar of sessions + embedded tmux client, focus keys
examples/tmux       tmux snippet binding prefix S / V to the picker
examples/workspaces sample definitions (the only workspace files tracked)
docs/               contributor docs
```

## Rules

- **Dependencies point inward.** `cmd` -> `restore`/`capture`/`pick`/`tui` -> `workspace`, `tmux`, `sshconfig`, `embed`. `workspace` imports nothing from this repo. `tmux` imports nothing from this repo. `project` -> `git`, `workspace` (config dir, workspace files for the task link); `git` imports nothing from this repo.
- **Only `internal/git` runs `git`.** Same reasoning: one place to validate and to fake.
- **Only `internal/tmux` runs `tmux`.** Everything else calls its methods. This keeps the socket-safety rule in one place and makes mocking possible.
- **Business logic does not live in `cmd`.** Command files parse flags, call a package, and print.
- **New subcommand = one new file in `cmd/vantage/` + one entry in `commands` (`main.go`).**
- Code style rules (file size, errors, interfaces, tests) are in `go-conventions.md`.
- **Everything is under `internal/`.** There is no public Go API yet.
- New packages need a one-sentence package comment saying what they own.

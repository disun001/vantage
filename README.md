# vantage

A terminal workspace manager built on tmux. Save tmux sessions as
**workspaces** (windows, panes, directories, SSH hosts), restore them with one
command, and browse them from a sidebar TUI.

**Status:** milestone 5 done. The workspace format, `vantage restore`, `vantage capture`, the ssh
split picker (`vantage pick`, tmux snippet in `examples/tmux`), and `vantage tui` (also just `vantage`) work.
The TUI shows sessions with their windows in a sidebar (`space` folds), embeds tmux (its own status bar is hidden),
and supports the mouse. `M-s` toggles focus, `M-h` / `M-l` move between sidebar and tmux.
Sidebar keys: `n` new workspace and `R` rename (each asks for the name in a pop-up), `d` delete the workspace file and kill its session (confirmed in a pop-up).
`H` switches the sidebar to your ssh hosts: `Enter` goes to the pane already connected to the host (a pop-up asks which when several are open) or opens ssh in a new window, `x` closes every open pane of the host after a `y`, `v` opens an ssh split beside the active pane and `s` one below it, `a` adds a host, `e` edits one in a pop-up (alias, host, user, port, jump; a new alias renames it, repoints other hosts' jumps and updates the saved workspaces), `d` deletes one after a `y` in a pop-up, warning there if workspaces or open panes use it (the open panes are closed and the saved workspaces lose those panes).
Changes go straight to `~/.ssh/config`, with a copy kept in `~/.ssh/config.vantage.bak`
(see [docs/architecture.md](docs/architecture.md)).

If a key does nothing, `VANTAGE_KEYLOG=/tmp/keys.log vantage` records each non-text key vantage receives and what it did to focus. Typed text is never logged.

## Quick start

```sh
make build                                   # ./bin/vantage
mkdir -p ~/.config/vantage/workspaces
cp examples/workspaces/demo.toml ~/.config/vantage/workspaces/
./bin/vantage list
./bin/vantage restore demo                   # creates the tmux session "demo"
tmux attach -t demo
```

Requires tmux and Go (see `go.mod` for the version). Contributing also needs [golangci-lint v2](https://golangci-lint.run/welcome/install/) for `make check`.

## Docs

| Doc                                                  | Read it for                                   |
| ---------------------------------------------------- | --------------------------------------------- |
| [AGENTS.md](AGENTS.md)                               | Hard rules. Read first                        |
| [docs/installation.md](docs/installation.md)         | Install, config location, dotfile managers    |
| [docs/workspace-format.md](docs/workspace-format.md) | TOML schema reference                         |
| [docs/architecture.md](docs/architecture.md)         | Design decisions and their reasons, roadmap   |
| [docs/layout.md](docs/layout.md)                     | Repo structure and where new code goes        |
| [docs/go-conventions.md](docs/go-conventions.md)     | Go style, errors, testing, file/package rules |
| [docs/workflow.md](docs/workflow.md)                 | Build, test, commit                           |
| [docs/contributing.md](docs/contributing.md)         | How to contribute                             |

# vantage

A terminal workspace manager built on tmux. Save tmux sessions as
**workspaces** (windows, panes, directories, SSH hosts), restore them with one
command, and browse them from a sidebar TUI.

**Status:** milestone 4 in progress. The workspace format, `vantage restore`, `vantage capture`, the ssh
split picker (`vantage pick`, tmux snippet in `examples/tmux`), and a first `vantage tui` (session sidebar with
an embedded tmux; `M-s` toggles focus) work. Mouse and scrollback are not done (see
[docs/architecture.md](docs/architecture.md)).

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

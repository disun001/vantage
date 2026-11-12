# Installation

## Build

```sh
git clone https://github.com/disun001/vantage && cd vantage
make build            # ./bin/vantage
# or: go install github.com/disun001/vantage/cmd/vantage@latest
```

Requires Go and tmux. Works on macOS and Linux/WSL.

## Config location

Workspace files are looked up in the first of:

| Order | Directory                             |
| ----- | ------------------------------------- |
| 1     | `$VANTAGE_CONFIG_DIR/workspaces`      |
| 2     | `$XDG_CONFIG_HOME/vantage/workspaces` |
| 3     | `~/.config/vantage/workspaces`        |

tmux socket: `--socket NAME` or `$VANTAGE_SOCKET`; default is your normal
tmux server.

## Dotfile managers

vantage reads plain files, so any dotfile manager works. With GNU Stow, put
`.config/vantage/workspaces/*.toml` in one of your packages and re-stow. Keep
your real workspaces there, not in this repo: they contain personal paths and
host aliases. Only `examples/` is tracked here.

## tmux bindings

`examples/tmux/vantage.conf` binds `prefix S` (stacked) and `prefix V` (side by
side) to a popup that asks for a local shell or an ssh host from `~/.ssh/config`.
Copy it next to your `tmux.conf` and add:

```tmux
source-file -q ~/.config/tmux/vantage.conf
```

The popup runs `vantage pick --split h|v` (see `vantage help`). Move with `j`/`k`, press `/` to fuzzy-filter, Enter to choose (the first entry is a local shell), Esc to go back or cancel. The right side shows the highlighted host (address, user, port, jump) and a green `●` marks hosts already connected in another pane or window. Only plain `ssh <alias>` connections are detected, and selecting still opens a new split. For a non-default socket add
`--socket NAME` or export `VANTAGE_SOCKET`.

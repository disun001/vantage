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

## tmux bindings (planned, milestone 3)

The `prefix S` / `V` picker bindings will ship as a snippet to `source-file`
from your `tmux.conf`. This section will be filled in then.

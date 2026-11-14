# Installation

Vantage needs tmux 3.2 or newer. It works on macOS and Linux (including WSL).

## Homebrew (recommended)

```sh
brew install disun001/tap/vantage
```

This installs tmux too. Update with `brew upgrade vantage`. To install it as part
of a dotfiles setup, add this to your `Brewfile`:

```ruby
tap "disun001/tap"
brew "vantage"
```

## Prebuilt binary

Download the archive for your OS and CPU from the
[releases page](https://github.com/disun001/vantage/releases), unpack it, and put
`vantage` somewhere on your `PATH`.

## go install

```sh
go install github.com/disun001/vantage/cmd/vantage@latest
```

Needs Go (version in `go.mod`). The binary lands in `$(go env GOPATH)/bin`,
which must be on your `PATH`.

## Build from source

```sh
git clone https://github.com/disun001/vantage && cd vantage
make build            # ./bin/vantage
```

Check the result with `vantage version`.

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

It also binds `prefix T` to name the pane (see [usage](usage.md#pane-names)). It replaces tmux's own set-pane-title key.

The popup runs `vantage pick --split h|v` (see `vantage help`). Move with `j`/`k`, press `/` to fuzzy-filter, Enter to choose (the first entry is a local shell), Esc to go back or cancel. The right side shows the highlighted host (address, user, port, jump) and a green `●` marks hosts already connected in another pane or window. A connection counts when the pane's foreground command is `ssh` to the host's alias or its real address; options like `-p` and a `user@` prefix are ignored. Selecting still opens a new split. For a non-default socket add
`--socket NAME` or export `VANTAGE_SOCKET`.

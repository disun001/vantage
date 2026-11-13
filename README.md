# vantage

A terminal workspace manager built on tmux. Save your tmux sessions as small
text files, restore them with one command, and jump between sessions and ssh
hosts from a sidebar.

## Why

tmux keeps your terminal alive, but it forgets your layout after a reboot and
gives you no overview of what is open. Vantage adds:

- **Workspaces.** Windows, panes, directories and ssh hosts saved as one
  readable TOML file per workspace. Restore it with `vantage restore`, or save a
  running session with `vantage capture`.
- **A sidebar.** Every session and its windows in one tree, next to a real tmux
  client. Your tmux keys, panes and plugins keep working.
- **ssh awareness.** Your `~/.ssh/config` hosts in the sidebar. See which are
  connected, jump to an open pane, or open a new ssh split from a pop-up.

## Install

macOS and Linux (including WSL), with [Homebrew](https://brew.sh):

```sh
brew install disun001/tap/vantage
```

This also installs tmux. Other options (`go install`, prebuilt binaries, build
from source) are in [docs/installation.md](docs/installation.md).

## Quick start

```sh
mkdir -p ~/.config/vantage/workspaces
curl -o ~/.config/vantage/workspaces/demo.toml \
  https://raw.githubusercontent.com/disun001/vantage/main/examples/workspaces/demo.toml

vantage list              # demo
vantage restore demo      # creates the tmux session "demo"
tmux attach -t demo
```

Or run `vantage` on its own to open the sidebar. Press `n` to create a
workspace, `H` to see your ssh hosts, `M-s` to switch focus between the
sidebar and tmux. The full key list is in [docs/usage.md](docs/usage.md).

To save a session you already have running:

```sh
vantage capture mysession
```

## Safety

- Editing an ssh host in the TUI changes `~/.ssh/config`. Vantage keeps a backup
  (`config.vantage.bak`), writes atomically and refuses to write anything it
  cannot parse back. Details in [docs/usage.md](docs/usage.md#safety).
- Commands in a workspace never run on restore unless you set `autorun = true`.
- Vantage makes no network connections. It only runs `tmux` and `ps`.

## Requirements

- tmux 3.2 or newer
- macOS, Linux, or Windows through WSL. Native Windows is not supported.

## Docs

| Doc                                                  | Read it for                                  |
| ---------------------------------------------------- | -------------------------------------------- |
| [docs/usage.md](docs/usage.md)                       | Commands, TUI keys, safety                   |
| [docs/installation.md](docs/installation.md)         | Install options, config location, tmux setup |
| [docs/workspace-format.md](docs/workspace-format.md) | TOML schema reference                        |
| [docs/architecture.md](docs/architecture.md)         | Design decisions and their reasons           |

## Contributing

Bug reports and ideas are welcome as issues. For anything bigger than a small
fix, please open an issue first so we can agree on the approach before you write
code. See [docs/contributing.md](docs/contributing.md). Report security
problems privately, as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)

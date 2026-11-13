# Usage

## Commands

| Command                     | Does                                                |
| --------------------------- | --------------------------------------------------- |
| `vantage`                   | Starts the TUI (same as `vantage tui`)              |
| `vantage list`              | Lists saved workspaces                              |
| `vantage restore <name>`    | Creates the tmux session for a workspace            |
| `vantage capture <session>` | Saves a running tmux session as a workspace file    |
| `vantage pick --split h\|v` | Splits the pane as local or ssh, chosen from a menu |
| `vantage version`           | Prints the version (`--version` works too)          |

`vantage --start` is the TUI with a startup step, meant for a terminal that
should always open on something (for example a terminal emulator's shell
command). If no tmux session is live it restores the first saved workspace
(alphabetically), or creates a bare `main` session when none are saved. Plain
`vantage` never creates anything on its own. Commands in the workspace still
follow `autorun`.

Every command that talks to tmux accepts `--socket NAME` (or set
`$VANTAGE_SOCKET`) to use a separate tmux server instead of your default one.

## The TUI

A sidebar on the left lists your tmux sessions and their windows. The right
side is a real tmux client, so panes, key bindings and persistence work as
usual. tmux's own status bar is hidden in the embedded view.

Focus keys (everything else goes straight to tmux):

| Key           | Does                                  |
| ------------- | ------------------------------------- |
| `M-s`         | Toggle focus between sidebar and tmux |
| `M-h` / `M-l` | Move between sidebar and tmux         |
| `M-/`         | Open the key help, from either side   |

The mouse works: click and scroll in the sidebar, and clicks, drags and wheel
over tmux are forwarded when tmux has `mouse on`.

The title pill shows the tmux mode: `VANTAGE` when idle, `PREFIX`, `COPY`,
`SYNC`, or `SIDEBAR` while the sidebar has focus.

### Sidebar keys (workspaces)

The sidebar footer shows only `? keys`. Press `?` in the sidebar (or `M-/` anywhere) for a pop-up
listing every key of the current view, grouped and explained. `esc`, `?` or
`q` closes it.

| Key     | Does                                                                    |
| ------- | ----------------------------------------------------------------------- |
| `space` | Fold or unfold a session                                                |
| `n`     | New workspace (asks for a name in a pop-up)                             |
| `R`     | Rename the workspace and its live session. On a window line: rename that window, in the saved file too                               |
| `K`     | Stop: kill the session, keep the workspace file, after a `y` in a pop-up |
| `d`     | Delete the workspace file and kill its session, after a `y` in a pop-up |
| `H`     | Switch the sidebar to your ssh hosts                                    |
| `P`     | Switch the sidebar to projects (details and tasks on the right)         |

### Projects view (after `P`)

Keys and forms are listed in [projects.md](projects.md#tui).

### Sidebar keys (ssh hosts, after `H`)

| Key     | Does                                                                                                                         |
| ------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `Enter` | Go to the pane already connected to the host (a pop-up asks which, if several), or open ssh in a new window                  |
| `x`     | Close every open pane of the host, after a `y`                                                                               |
| `v`     | Open an ssh split beside the active pane                                                                                     |
| `s`     | Open an ssh split below the active pane                                                                                      |
| `a`     | Add a host                                                                                                                   |
| `e`     | Edit a host: alias, host, user, port, jump. A new alias renames it, repoints other hosts' jumps and updates saved workspaces |
| `d`     | Delete a host after a `y`. The pop-up warns if workspaces or open panes use it                                               |

Host changes are written to `~/.ssh/config` (see [Safety](#safety)).

## ssh split picker

`examples/tmux/vantage.conf` binds `prefix S` (stacked) and `prefix V` (side by
side) to a pop-up that asks for a local shell or an ssh host. See
[installation](installation.md#tmux-bindings).

## Safety

- Editing a host writes `~/.ssh/config` directly. Vantage keeps a copy in
  `~/.ssh/config.vantage.bak`, writes atomically, re-parses the result before
  replacing the file, and touches only the `Host` block you edited.
- It refuses to write when the file has CRLF line endings, does not parse,
  uses a shared `Host a b` line, or the change would alter other hosts.
- Deleting or renaming a host rewrites the saved workspace files that use it.
  Each rewritten file keeps a `name.toml.vantage.bak` copy.
- Commands in a workspace only run on restore when you set `autorun = true`.
  `capture` saves every command with `autorun = false`.
- Vantage makes no network connections. It runs `tmux` and `ps`.

## Troubleshooting

If a key does nothing, `VANTAGE_KEYLOG=/tmp/keys.log vantage` records each
non-text key vantage receives and what it did to focus. Typed text is never
logged.

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

| Key     | Does                                                                                                   |
| ------- | ------------------------------------------------------------------------------------------------------ |
| `space` | Fold or unfold a session                                                                               |
| `n`     | New workspace (asks for a name in a pop-up)                                                            |
| `R`     | Rename the workspace and its live session. On a window line: rename that window, in the saved file too |
| `K`     | Stop: kill the session, keep the workspace file, after a `y` in a pop-up                               |
| `d`     | Delete the workspace file and kill its session, after a `y` in a pop-up                                |
| `H`     | Switch the sidebar to your ssh hosts                                                                   |
| `P`     | Switch the sidebar to projects (repo details on the right)                                             |

### Projects view (after `P`)

Keys and forms are listed in [projects.md](projects.md#tui).

### Sidebar keys (ssh hosts, after `H`)

| Key     | Does                                                                                                                              |
| ------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `Enter` | Go to the pane already connected to the host (a pop-up asks which, if several), or open ssh in a new window                       |
| `x`     | Close every open pane of the host, after a `y`                                                                                    |
| `v`     | Open an ssh split beside the active pane                                                                                          |
| `s`     | Open an ssh split below the active pane                                                                                           |
| `a`     | Add a host                                                                                                                        |
| `e`     | Edit a host: alias, host, user, port, jump, key. A new alias renames it, repoints other hosts' jumps and updates saved workspaces |
| `d`     | Delete a host after a `y`. The pop-up warns if workspaces or open panes use it                                                    |

The right pane shows the host's details instead of a terminal: its ssh options,
the key, whether a password is stored (never the password itself), the open
panes connected to it and the saved workspaces that use it. After a connect or
a split, the sidebar goes back to workspaces.

| Key | Does                                                            |
| --- | --------------------------------------------------------------- |
| `u` | Unlock the password vault (asked once per run, or on first use) |

Host changes are written to `~/.ssh/config` (see [Safety](#safety)).

### Key and password login

The add / edit form has a `key` line, the path of a private key. It is written
as `IdentityFile <path>` plus `IdentitiesOnly yes`, and clearing it removes both.
Vantage stores the path only, never the key. A key with its own passphrase is
left to ssh and your agent.

With a vault configured, the form also has a `password` section, below the ssh
config part. The two parts save apart: `Enter` saves only the section the
cursor is in, so one failing never half-saves the other. The password section
opens once the host exists. `ctrl+x` on it removes the stored password.

- **Where.** `passwords.age` in the config dir (`$VANTAGE_CONFIG_DIR`, else
  `$XDG_CONFIG_HOME/vantage`, else `~/.config/vantage`): one file, encrypted with
  [age](https://age-encryption.org), safe to keep in a dotfiles repo.
- **Unlock.** By default a passphrase, chosen twice when the vault is created and
  asked once per run. Set `VANTAGE_AGE_IDENTITY` to an age identity file to unlock
  without a prompt.
- **Use.** For a host with a stored password, vantage starts ssh with
  `SSH_ASKPASS` pointing at itself (`SSH_ASKPASS_REQUIRE=force`, OpenSSH 8.4 or
  newer) and `NumberOfPasswordPrompts=1`. The password never appears in the command
  line, the shell history or the environment: the helper asks the running TUI over
  a socket only your user can reach. It answers only ssh's plain
  `user@host's password:` prompt for that host. Anything else (host key question,
  key passphrase, verification code, a jump host's password) goes to you in the
  pane. One wrong attempt ends the connection, and the pane says the stored
  password may be wrong.
- **Where it applies.** Connecting from the hosts view (`Enter`, `v`, `s`) uses
  the vault the TUI has open. Restored panes (`vantage restore`, or a restore
  from the TUI) and the `prefix S` / `V` picker find the running TUI instead: it
  also listens on `$XDG_RUNTIME_DIR/vantage/askpass.sock` (else
  `<tmpdir>/vantage-<uid>/askpass.sock`, a directory only you can enter). So one
  unlock covers every pane, and a host with no stored password runs plain ssh.
  With a passphrase vault the socket exists once you have unlocked it; an
  identity vault opens at start. Only one TUI serves that socket; a second one
  keeps a private socket for its own panes.
- **No TUI running.** The helper opens the vault itself: it asks for the vault
  passphrase in that pane (empty answer: type the ssh password instead), or uses
  `VANTAGE_AGE_IDENTITY` with no prompt. It cannot know which hosts have a
  password without opening the vault, so a host with none costs one extra
  passphrase question. With no vault file, panes run plain ssh. A pane that uses
  the helper allows one password attempt (`NumberOfPasswordPrompts=1`), also
  when you type the password.
- **Rename and delete.** A rename moves the stored password to the new alias. A
  delete removes it, and the confirmation says so.

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
- Stored passwords live only in the encrypted vault and, while the TUI runs, in
  its memory. They are never written to a command line, a log or `~/.ssh/config`.
- Vantage makes no network connections. It runs `tmux`, `ps` and `ssh` (for the
  panes you open).

## Troubleshooting

If a key does nothing, `VANTAGE_KEYLOG=/tmp/keys.log vantage` records each
non-text key vantage receives and what it did to focus. Typed text is never
logged.

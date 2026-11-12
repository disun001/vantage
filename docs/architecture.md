# Architecture

vantage wraps tmux. tmux stays the multiplexer and the source of truth for live
state. vantage adds saved workspaces, an SSH-aware picker, and (planned) a
sidebar TUI.

## Decisions

| Topic            | Decision                                                                                                                                | Why                                                                                                |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| Relation to tmux | A wrapper TUI: sidebar on the left, embedded tmux on the right                                                                          | One always-visible sidebar, while keeping tmux's panes, keys, and persistence                      |
| Rendering        | Embedded terminal emulator: attach a real tmux client in a PTY and draw it                                                              | tmux draws panes and borders. Much less work than control mode (`-CC`), which can be adopted later |
| Stack            | Go: Bubble Tea, `charmbracelet/x/vt`, `creack/pty`                                                                                      | One static binary. A spike confirmed truecolor, wide characters, splits, and vim                   |
| Workspace model  | TOML definition is the source of truth. `capture` writes it from a live session                                                         | Hand-editable and dotfile-friendly, with a shortcut for the common case                            |
| Storage          | One TOML per workspace                                                                                                                  | Easy diffs and sharing. See `workspace-format.md`                                                  |
| SSH hosts        | Read `~/.ssh/config` with `kevinburke/ssh_config`. Panes run `ssh <alias>`                                                              | No duplicated host list. Read-only first                                                           |
| Keys             | The tmux prefix (`C-a`) and all other keys pass straight to tmux. The TUI intercepts only focus keys: edge-aware `M-h` and `M-s` toggle | No new prefix to learn. `M-s` guarantees an escape from panes that swallow keys                    |
| tmux server      | Default server, with a configurable socket. Tests always use an isolated socket                                                         | Existing sessions show up and can be captured. Isolation for development                           |
| SSH split        | `prefix S` / `V` open a `display-popup` picker (local or ssh host). `s` / `v` stay local                                                | Works inside and outside the TUI                                                                   |
| Capture          | cwd and ssh detection. Other commands are saved as `cmd` with `autorun = false`                                                         | Nothing runs by surprise                                                                           |
| Switching        | One embedded client, `tmux switch-client`                                                                                               | Simplest. tmux owns all state                                                                      |
| Sidebar          | Name, live dot, window and pane counts, ssh badge. A tree view later                                                                    | Useful at a glance with one cheap query                                                            |

## Roadmap

| #   | Milestone                                                          | Status  |
| --- | ------------------------------------------------------------------ | ------- |
| 1   | Workspace TOML schema + `restore`                                  | Done    |
| 2   | `capture` (live session to TOML, ssh detection, `autorun = false`) | Done    |
| 3   | ssh config reader + `pick` popup + tmux bindings (`S` / `V`)       | Done    |
| 4   | TUI shell: sidebar + embedded tmux + focus keys + socket config    | First slice done (`vantage tui`) |
| 5   | Tree view, ssh add and edit                                        | Planned |

## Untested risks for the TUI (milestone 4)

First slice (`vantage tui`) covers: session sidebar, embedded client,
`switch-client`, restore of saved workspaces from the sidebar, `M-s` toggle,
edge-aware `M-h`, key and paste passthrough, resize.

Still untested or missing: mouse (not forwarded), scrollback, copy-mode, fast
output, `M-h` is the only focus key into the sidebar (no `M-l` back yet), and
the tmux status bar of the inner client is still drawn.

## Safety

Anything that can kill or rewrite the user's real tmux or ssh state is guarded
in code, not only by convention. See `AGENTS.md`.

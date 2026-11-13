# AGENTS.md

vantage: Go tool that manages tmux workspaces. Human intro: `README.md`. Do not duplicate it here.

Rules for anyone changing this code, human or AI coding agent. Agents read this file first.

## Hard rules

1. **Never touch the user's default tmux server in tests or experiments.** Always use a dedicated socket (`tmux -L <name>`). `tmux.Client.KillServer` refuses an empty socket on purpose; never bypass it. Never run bare `tmux kill-server`.
2. **Never read or print `~/.ssh` contents** (keys, known_hosts, config values beyond host aliases/options we parse on purpose). Tests use fixtures, not the real file.
3. **Validate anything that reaches a shell.** ssh aliases and session names are checked in `workspace.Validate`; keep it that way when adding fields. Commands only run when the user opted in (`autorun = true`).
4. **Both OSes.** macOS and Linux/WSL. No hardcoded `/Users/...`, `/opt/homebrew`, `/home/...`. Config is `$VANTAGE_CONFIG_DIR` > `$XDG_CONFIG_HOME/vantage` > `~/.config/vantage` (not `os.UserConfigDir`, which is wrong on macOS).
5. **Personal data stays out of the repo.** Real workspace files live in the user's config dir. Only `examples/` is tracked. No real hostnames, usernames, emails or paths in code, tests, docs or examples.
6. **Verify before committing:** `make check` must pass.
7. **Git:** small commits, Conventional Commits. Agents never push, tag or open a pull request without the maintainer's explicit OK. No `Co-Authored-By` trailer.

## Read on demand

| Doing this                                                              | Read                       |
| ----------------------------------------------------------------------- | -------------------------- |
| Writing or reviewing any Go code (file size, errors, tests, interfaces) | `docs/go-conventions.md`   |
| Adding a package, deciding where code goes                              | `docs/layout.md`           |
| Changing the TOML schema                                                | `docs/workspace-format.md` |
| Touching tmux calls, TUI, keys, roadmap                                 | `docs/architecture.md`     |
| Build, test, commit                                                     | `docs/workflow.md`         |

## Commands

`make build` · `make test` · `make lint` · `make fmt` · `make check` (needs golangci-lint v2)

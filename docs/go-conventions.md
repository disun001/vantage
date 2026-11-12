# Go conventions

Rules for keeping the code easy to change. `make check` enforces what it can
(see the table); the rest is review.

| Rule                                                            | Enforced by                                  |
| --------------------------------------------------------------- | -------------------------------------------- |
| Formatting and import grouping                                  | `gofumpt`, `goimports` (via `golangci-lint`) |
| No `init()`                                                     | `gochecknoinits`                             |
| Function length (60 lines / 40 statements)                      | `funlen` (not in tests)                      |
| Errors wrapped with `%w`, compared with `errors.Is/As`          | `errorlint`                                  |
| Doc comments on exported identifiers                            | `revive`                                     |
| Unchecked errors, suspicious code, unused code                  | `errcheck`, `staticcheck`, `govet`, `unused` |
| `//nolint` must name a linter and explain why                   | `nolintlint`                                 |
| File size, package naming, interface placement, everything else | Review                                       |

Config: `.golangci.yml`. Do not silence a linter with `//nolint` to get past a
finding; fix the code, or change the config in its own PR with the reason.

## Files and packages

- **One responsibility per file; name the file after it.** A CLI command is one file (`restore.go`), its flag helpers live in `flags.go`. If you need "and" to describe a file, split it.
- **Soft limit: ~300 lines per file, ~50 lines per function.** Past that, split by responsibility, not by arbitrary line count.
- **`main.go` only dispatches.** Commands live in `cmd/vantage/<name>.go` and register in the `commands` table in `main.go`. Adding a command is adding one file plus one entry.
- **No `util`, `common`, `helpers`, or `misc` packages.** Name packages for what they own (`workspace`, `tmux`, `restore`). A helper used by one package stays in that package.
- **Every package has a package comment** saying what it owns and, if relevant, what it must not do.
- Dependency direction is in `layout.md`; do not create import cycles to work around it, move the code.

## Design

- **Accept interfaces, return concrete types.** Define a small interface in the package that consumes it, only when there is a second implementation or a test needs a fake. Do not pre-declare interfaces next to their only implementation.
- **No package-level mutable state, no `init()`.** Pass dependencies in (e.g. a `tmux.Client` value). The only package-level vars allowed are read-only tables like `commands`.
- **Keep `os` access at the edge.** Reading env vars, `os.Args`, and writing to stdout happen in `cmd/`. Library packages take values and `io.Writer`s. (`workspace.Dir` reads env on purpose, as the single config-location lookup.)
- **Make behaviour testable without a terminal.** Commands take `(args []string, out io.Writer)`; `run()` returns an exit code instead of calling `os.Exit`.
- **Prefer small structs of plain data** (`Workspace`, `Window`, `Pane`) with behaviour in functions in the owning package.

## Errors

- Return errors; do not `panic` outside truly unreachable code. No `log.Fatal` outside `main`.
- **Wrap with context and `%w`**: `fmt.Errorf("window %q layout: %w", name, err)`. Messages are lowercase, no trailing period, and say what was being done.
- Handle an error once: either return it or act on it, never both (no log-and-return).
- Errors shown to users come from `cmd/` (`vantage: ...`). Libraries return them untouched.
- **`fmt.Fprint*` errors may be ignored only when writing to stdout or stderr** (`.golangci.yml` excludes them from `errcheck`). When writing to a file or any other destination (workspace files, config), check the error, because a failed write there means lost data.

## Safety (repo-specific)

- Everything that execs tmux goes through `internal/tmux`. Everything reaching a shell or a tmux target is validated first (`workspace.Validate`).
- Tests never touch the default tmux server (see `workflow.md`).

## Naming and style

- `gofmt` formatting is mandatory (`make fmt`). Short receiver names, no stuttering (`workspace.Load`, not `workspace.LoadWorkspace`).
- Exported identifiers have a doc comment starting with their name. Comments explain **why**, not what.
- Constants and regexes with non-obvious intent get a comment saying why they are that strict.

## Tests

- **Table-driven** for pure logic (see `workspace_test.go`). Use `t.Helper()`, `t.TempDir()`, `t.Setenv()`, `t.Cleanup()`.
- Test behaviour through the public API of the package. Add a failing test first when fixing a bug.
- Tests needing tmux skip when it is missing and use an isolated socket.
- Test bad input on anything that reaches a shell, a path, or a tmux target.

## Dependencies

- Prefer the standard library. Add a third-party module only for a clear need (TOML, ssh config parsing, TUI); mention why in the PR.
- Run `go mod tidy` after changing imports.

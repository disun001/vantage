# Workflow

## Commands

| Command      | Does                                                |
| ------------ | --------------------------------------------------- |
| `make build` | Builds `./bin/vantage`                              |
| `make test`  | Runs all tests                                      |
| `make lint`  | golangci-lint plus a gofumpt/goimports format check |
| `make fmt`   | Formats with gofumpt and goimports                  |
| `make gen`   | Regenerates sqlc code in `internal/store/gen`       |
| `make check` | lint, `go vet`, tests, sqlc drift. Must pass first  |

`lint`, `fmt`, and `check` need [golangci-lint v2](https://golangci-lint.run/welcome/install/) on your `PATH` (config in `.golangci.yml`).

## Testing with tmux

- Tests that need tmux use a throwaway socket (`vantage-test-<pid>-<nanos>`) and kill only that server in `t.Cleanup`. Copy `testClient` in `internal/restore/restore_test.go`.
- They skip themselves if tmux is not installed.
- Never run a test or script against the default server. `tmux.Client.KillServer` refuses an empty socket.
- Never put real `~/.ssh` contents in tests. Use fixtures.

To try the CLI by hand without touching your sessions:

```sh
export VANTAGE_CONFIG_DIR=$(mktemp -d)   # scratch workspaces
./bin/vantage restore demo --socket scratch
tmux -L scratch attach        # look around
tmux -L scratch kill-server   # clean up
```

## Testing chat

- Chat tests run against a fake `claude` that emits canned stream-json (`docs/chat.md`). They never call the real CLI, so `make check` uses no subscription.
- One live test runs the real `claude -p`. It runs only with `VANTAGE_LIVE_CLAUDE=1`. Run it by hand before releases.
- To try the UI without the subscription: `VANTAGE_CLAUDE=internal/chat/testdata/fake-claude.sh FAKE_CLAUDE_SCENARIO=tooluse VANTAGE_STATE_DIR=$(mktemp -d) ./bin/vantage chat new`.
- Database tests set `VANTAGE_STATE_DIR` to a temp dir. Never use the real state dir.

## Markdown

Docs are formatted with [prettier](https://prettier.io) (`.prettierrc` at the repo root). Most editors format on save; to check by hand: `npx prettier --check "*.md" "docs/*.md"`. Prettier aligns table columns and adds blank lines after headings, so run it before committing doc changes to avoid noisy diffs.

## Commits

Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`).
Small, focused commits. Run `make check` first.

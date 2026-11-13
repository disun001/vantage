# Chat

Status: **slice 2 built** (`internal/chat`, `internal/store`, `internal/chatui`, `vantage chat`). Slices are at the bottom.

Native AI chat inside vantage. Claude only at first. A chat is a Bubble Tea
program (`vantage chat`) that runs in a normal tmux pane and drives the
`claude` CLI headless. Chats are stored in SQLite (see `storage.md`) so the
sidebar and, later, hooks can read them.

## Decisions

| Topic            | Decision                                                                                                                | Why                                                                                         |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| Where it runs    | `vantage chat` in a tmux pane. No daemon                                                                                | tmux already outlives the TUI and owns the process. Smallest step, fits "tmux is the truth" |
| Library first    | The core is `internal/chat`. It does not assume a pane                                                                  | A later daemon or other front end can reuse it                                              |
| Transport        | One long-lived `claude -p --input-format stream-json --output-format stream-json --verbose` child per chat              | Real chat feel, structured events, reuses Claude Code auth and tools. No Node, no API key   |
| Recovery         | If the child dies, respawn with `--resume <session_id>`                                                                 | Claude keeps the conversation. We only restart the process                                  |
| Provider seam    | `chat.Provider` interface. Only `claude` implements it                                                                  | A second provider (for example ACP) can be added without touching the UI or store           |
| Permissions      | `--permission-mode plan`. Read-only, no edits, no shell                                                                 | Matches the orchestrator. Chat plans, agents act. Approval UI is a later slice              |
| Auth             | Subscription login. `ANTHROPIC_API_KEY` is removed from the child's environment unless `--allow-api-key` is set         | `-p` always prefers an exported key and would bill API credits silently                     |
| Ownership        | A chat is a pane in a session. The chat id (in SQLite) is the stable key. The tmux pane id is a live pointer only       | Pane ids change on restore                                                                  |
| Remote           | Local only. `chats.host` is reserved (NULL = local)                                                                     | Remote claude means remote login, ssh streaming, remote resume. Its own project             |
| Events           | Normalized `kind` and `text` plus `raw`. Final messages and tool events only. No per-token rows                         | Hooks and sidebar query without parsing. Raw keeps everything if the mapper is wrong        |
| Entry points     | A `Claude chat (local)` row in the `vantage pick` menu (prefix `S` / `V`), and the `vantage chat` CLI. Plain `v` is unchanged | Fits the existing split workflow                                                         |

## Process model

```text
tmux pane ── vantage chat <id> ── claude -p (stream-json, plan mode)
                  │
                  └── SQLite (chats, events)  ◄── sidebar, hooks (later)
```

- `vantage chat new` creates a `chats` row, then runs the chat in the current
  pane. `vantage chat <id>` reopens an existing one and replays its events.
- The chat process owns the child. If the pane or the process dies, the child
  dies. The next `vantage chat <id>` respawns it with `--resume`.
- Inside the pane the cwd is the pane's directory when the chat was created.
  It is stored on the row and used again on resume.

## Event mapping

Claude stream-json lines are mapped on write (`internal/chat/claude_map.go`).
Shapes were confirmed against a real transcript:
`internal/chat/testdata/tooluse.jsonl`. The child emits `init` again on every
turn, and one child serves many turns while stdin stays open.

| Claude line                        | Stored `kind`    | Notes                                              |
| ---------------------------------- | ---------------- | -------------------------------------------------- |
| user message sent by us            | `user_msg`       | written before the turn starts                     |
| `system` init                      | `session`        | carries `session_id`, saved on the chat row        |
| `assistant` text block             | `assistant_msg`  | the full message, not deltas                       |
| `assistant` `tool_use` block       | `tool_use`       | name and a short argument summary in `text`        |
| `user` `tool_result` block         | `tool_result`    | truncated in `text`, full content in `raw`         |
| `result`                           | `turn_end`       | cost, usage and `is_error` in `raw`                |
| spawn failure, bad JSON, exit      | `error`          | `text` has the reason                              |

The first event of each line keeps the original line in `raw`. Unknown line
types are stored with `kind = 'unknown'` and never dropped. Only known noise
is skipped: `hook_started`, `hook_response`, `commands_changed` (tens of KB
per turn) and `rate_limit_event`. The `session` event text is
`model=... auth=...`; `apiKeySource: none` means the subscription.

## Using it

- `vantage chat new` starts a chat in the current directory and pane.
  `vantage chat <id>` reopens one and replays it. `vantage chat list` prints ids.
- In `vantage pick` (prefix `S` / `V`) the `Claude chat (local)` row splits the
  pane and runs `vantage chat new` in it. The pane closes when the chat quits.
- Keys: `enter` sends, `alt+enter` or `ctrl+j` adds a newline, `pgup`/`pgdn`
  scroll, `esc` or `ctrl+c` interrupts a running turn, `ctrl+c` on an idle
  chat or `ctrl+d` quits.
- Flags: `--cwd`, `--turn-timeout`, `--allow-api-key`. `VANTAGE_CLAUDE` points
  at another claude executable (tests use the fake).
- `ctrl+o` shows or hides tool results (first 8 lines each). Assistant text is
  rendered as markdown. The header shows model, auth, the latest turn's context
  size and total output tokens.
- Text appears per whole message, not per token (see Events).

## Turns and status

`chats.status` is one of `idle`, `running`, `error`, `interrupted`.

- Sending a message sets `running`. `turn_end` sets `idle`.
- When a chat opens and finds `running` with no live child, it marks the turn
  `interrupted` (writes an `error` event) and respawns with `--resume`.
- The interrupt key kills the child, marks the turn `interrupted`, and
  respawns. There is no mid-turn cancel protocol.
- A turn timeout is configurable. No default is chosen yet.

## Auth

- Run `claude /login` once. `-p` then uses the subscription limits.
- The child environment drops `ANTHROPIC_API_KEY` by default. The chat header
  shows the auth source (`subscription` or `api key`).
- Never use `--bare`. It skips the subscription login.
- Anthropic's limits assume individual use. If vantage is ever shipped to
  other people, re-check the current terms.

## Slices

| Slice | Scope                                                                                                              |
| ----- | ------------------------------------------------------------------------------------------------------------------ |
| 0 (done) | `internal/chat` (spawn, parse, `Provider`), `internal/store` (schema, goose, sqlc), fake `claude`, first real fixture |
| 1 (done) | `vantage chat`: bare transcript UI (whole messages, tool calls as one-line markers, multi-line input), picker row, resume |
| 2 (done) | Markdown rendering (glamour), tool results toggled with `ctrl+o`, header with model, auth source and tokens       |
| 3     | Sidebar shows each chat's state from SQLite                                                                        |
| Later | `[[chat]]` entry in the workspace file for restore, hooks (chat to task), remote host chats, permission prompts, more providers |

## Testing

- A fake `claude` executable (Go test binary or script in `testdata/`) emits
  canned stream-json. `internal/chat` takes the executable and its args from
  config, so tests point at the fake. Scenarios: normal turn, tool use, crash
  mid-turn, hang, invalid JSON.
- One live test runs the real `claude -p`. It is skipped unless
  `VANTAGE_LIVE_CLAUDE=1`, so `make check` never uses the subscription.
- Tests never touch the real `claude` login, the real state dir (use
  `VANTAGE_STATE_DIR`), or the default tmux server.

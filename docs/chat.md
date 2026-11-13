# Chat

Status: **planned**. Nothing here is built yet. Slices are at the bottom.

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

Claude stream-json lines are mapped on write. The exact shapes must be
confirmed against a real captured transcript (slice 0, first fixture).

| Claude line                        | Stored `kind`    | Notes                                              |
| ---------------------------------- | ---------------- | -------------------------------------------------- |
| user message sent by us            | `user_msg`       | written before the turn starts                     |
| `system` init                      | `session`        | carries `session_id`, saved on the chat row        |
| `assistant` text block             | `assistant_msg`  | the full message, not deltas                       |
| `assistant` `tool_use` block       | `tool_use`       | name and a short argument summary in `text`        |
| `user` `tool_result` block         | `tool_result`    | truncated in `text`, full content in `raw`         |
| `result`                           | `turn_end`       | cost, usage and `is_error` in `raw`                |
| spawn failure, bad JSON, exit      | `error`          | `text` has the reason                              |

Every row keeps the original line in `raw`. Unknown line types are stored
with `kind = 'unknown'` and never dropped.

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
| 0     | `internal/chat` (spawn, parse, `Provider`), `internal/store` (schema, goose, sqlc), fake `claude`, first real fixture |
| 1     | `vantage chat`: bare transcript UI (streamed text, tool calls as one-line markers, multi-line input), picker row, resume |
| 2     | Markdown rendering (glamour), collapsible tool calls, status line (model, auth source, tokens)                     |
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

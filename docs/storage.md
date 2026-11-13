# Storage

Status: **slice 0 built**. Workspaces stay TOML files (`workspace-format.md`). This
doc covers the SQLite database that chat introduces.

## Decisions

| Topic       | Decision                                                                                   | Why                                                              |
| ----------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------- |
| Engine      | SQLite through `modernc.org/sqlite` (pure Go)                                              | No cgo, so macOS and Linux/WSL builds stay simple                |
| Location    | `$VANTAGE_STATE_DIR` > `$XDG_STATE_HOME/vantage` > `~/.local/state/vantage/vantage.db`      | History is state, not config. Keeps it out of dotfiles repos     |
| Migrations  | `pressly/goose/v3`, numbered SQL files embedded in the binary, run on open                 | Same tooling as the orchestrator. Versioned and reviewable       |
| Queries     | `sqlc` (sqlite engine). Generated code is committed                                        | Compile-time checked SQL, and orchestrator queries port easily   |
| Concurrency | WAL mode, `busy_timeout` set, one connection per process                                   | The chat panes, the sidebar and the CLI are separate processes   |
| Schema      | Fresh and minimal. Borrow patterns from the orchestrator, not its tables                   | Its schema is tied to projects, PRs and sessions we do not have  |

Path rules follow `AGENTS.md` rule 4: no hardcoded home paths, no
`os.UserConfigDir`. Resolve the directory in one function (like
`workspace.Dir`), create it with mode `0700`, and the file with `0600`.
Transcripts may contain private code, so they never go in the repo or in
workspace files.

## Schema (slice 0)

```sql
CREATE TABLE chats (
  id                 TEXT PRIMARY KEY,          -- stable key, passed to `vantage chat <id>`
  provider           TEXT NOT NULL,             -- 'claude'
  provider_session   TEXT,                      -- claude session_id, for --resume
  session_name       TEXT,                      -- tmux session it was opened in
  pane_id            TEXT,                      -- live pointer, may be stale
  cwd                TEXT NOT NULL,
  host               TEXT,                      -- NULL = local. Reserved
  status             TEXT NOT NULL,             -- idle | running | error | interrupted
  created_at         INTEGER NOT NULL,          -- unix ms
  updated_at         INTEGER NOT NULL
);

CREATE TABLE events (
  id       INTEGER PRIMARY KEY,
  chat_id  TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  seq      INTEGER NOT NULL,                    -- order within the chat
  turn     INTEGER NOT NULL,
  ts       INTEGER NOT NULL,                    -- unix ms
  kind     TEXT NOT NULL,                       -- see chat.md event mapping
  text     TEXT NOT NULL DEFAULT '',
  raw      TEXT,                                -- original provider line
  UNIQUE (chat_id, seq)
);
CREATE INDEX events_chat_seq ON events(chat_id, seq);
```

The columns can change before slice 0 lands. After it ships, change the schema
only with a new goose migration. `0002` added `chats.tmux_socket` (the tmux
server name, since pane ids repeat across servers).

## Layout and workflow

- Package: `internal/store` owns the connection, migrations and generated
  queries. Nothing else imports the driver.
- Migrations: `internal/store/migrations/NNNN_name.sql`, embedded.
- Queries: `internal/store/queries/*.sql`. Generated Go goes to
  `internal/store/gen/` and is committed. Never edit generated files.
- `make gen` runs `sqlc generate` (pinned as a `tool` in `go.mod`, no install
  needed). `make check` fails if regenerating changes the tree.

## Rules

- Tests use a temp dir through `VANTAGE_STATE_DIR`. Never the real state dir.
- Writers append events in a transaction with the status update that goes
  with them.
- Do not store the full workspace or ssh data here. The TOML files stay the
  source of truth for workspaces.
- Raw provider lines may be large. Prune by size later if needed. Never
  prune the normalized columns.

-- +goose Up
CREATE TABLE chats (
  id                 TEXT PRIMARY KEY,
  provider           TEXT NOT NULL,
  provider_session   TEXT,
  session_name       TEXT,
  pane_id            TEXT,
  cwd                TEXT NOT NULL,
  host               TEXT,
  status             TEXT NOT NULL,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);

CREATE TABLE events (
  id       INTEGER PRIMARY KEY,
  chat_id  TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  seq      INTEGER NOT NULL,
  turn     INTEGER NOT NULL,
  ts       INTEGER NOT NULL,
  kind     TEXT NOT NULL,
  text     TEXT NOT NULL DEFAULT '',
  raw      TEXT,
  UNIQUE (chat_id, seq)
);
CREATE INDEX events_chat_seq ON events(chat_id, seq);

-- +goose Down
DROP TABLE events;
DROP TABLE chats;

-- name: AppendEvent :exec
INSERT INTO events (chat_id, seq, turn, ts, kind, text, raw)
VALUES (
  sqlc.arg(chat_id),
  (SELECT COALESCE(MAX(seq), 0) + 1 FROM events WHERE events.chat_id = sqlc.arg(chat_id)),
  sqlc.arg(turn), sqlc.arg(ts), sqlc.arg(kind), sqlc.arg(text), sqlc.arg(raw)
);

-- name: ListEvents :many
SELECT * FROM events WHERE chat_id = ? ORDER BY seq;

-- name: LastTurn :one
SELECT CAST(COALESCE(MAX(turn), 0) AS INTEGER) FROM events WHERE chat_id = ?;

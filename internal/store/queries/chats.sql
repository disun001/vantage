-- name: CreateChat :exec
INSERT INTO chats (id, provider, session_name, pane_id, tmux_socket, cwd, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 'idle', ?, ?);

-- name: GetChat :one
SELECT * FROM chats WHERE id = ?;

-- name: ListChats :many
SELECT * FROM chats ORDER BY updated_at DESC;

-- name: SetChatStatus :exec
UPDATE chats SET status = ?, updated_at = ? WHERE id = ?;

-- name: SetChatSession :exec
UPDATE chats SET provider_session = ?, updated_at = ? WHERE id = ?;

-- name: SetChatPane :exec
UPDATE chats SET pane_id = ?, session_name = ?, tmux_socket = ?, updated_at = ? WHERE id = ?;

-- name: ListSocketPanes :many
SELECT pane_id, status FROM chats WHERE tmux_socket = ? AND pane_id IS NOT NULL;

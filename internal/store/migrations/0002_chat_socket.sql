-- +goose Up
-- Pane ids (%3) repeat across tmux servers, so a chat records which server
-- (socket name) its pane lives on.
ALTER TABLE chats ADD COLUMN tmux_socket TEXT;

-- +goose Down
ALTER TABLE chats DROP COLUMN tmux_socket;

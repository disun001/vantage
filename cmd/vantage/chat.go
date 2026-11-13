package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/disun001/vantage/internal/chat"
	"github.com/disun001/vantage/internal/chatui"
	"github.com/disun001/vantage/internal/store"
)

var chatCmd = command{
	name:    "chat",
	usage:   "chat new|list|<id>",
	summary: "claude chat in this pane (new starts one, <id> resumes it)",
	run:     runChat,
}

// chatIDRe is what newChatID generates. Ids reach a command line and a
// database key, so nothing else is accepted.
var chatIDRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

func runChat(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "working directory for a new chat (default: current directory)")
	timeout := fs.Duration("turn-timeout", 0, "abort a turn that runs longer than this (0 = never)")
	allowKey := fs.Bool("allow-api-key", false, "keep ANTHROPIC_API_KEY for claude (bills the API, not the subscription)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: vantage chat new|list|<id>")
	}
	dir, err := store.StateDir()
	if err != nil {
		return err
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dir)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	if pos[0] == "list" {
		return listChats(ctx, st, out)
	}
	id := pos[0]
	if id == "new" {
		if id, err = createChat(ctx, st, *cwd); err != nil {
			return err
		}
	} else if !chatIDRe.MatchString(id) {
		return fmt.Errorf("%q is not a chat id (see vantage chat list)", id)
	}
	provider := chat.Claude{Exe: os.Getenv("VANTAGE_CLAUDE"), Env: os.Environ(), AllowAPIKey: *allowKey}
	return runChatView(ctx, st, provider, id, *timeout)
}

func runChatView(ctx context.Context, st *store.Store, p chat.Provider, id string, timeout time.Duration) error {
	row, err := st.Chat(ctx, id)
	if err != nil {
		return err
	}
	pane, socket := tmuxPlace()
	if err := st.SetPane(ctx, id, "", pane, socket); err != nil {
		return err
	}
	return chatui.Run(ctx, os.Stdin, os.Stdout, chatui.Config{
		Title: fmt.Sprintf("claude %s  %s", id[:8], row.Cwd),
		Open: func(onEvent func(store.Event)) (chatui.Driver, error) {
			return chat.Open(ctx, st, p, id, chat.Options{TurnTimeout: timeout, OnEvent: onEvent})
		},
		History: func(ctx context.Context) ([]store.Event, error) { return st.Events(ctx, id) },
	})
}

func createChat(ctx context.Context, st *store.Store, cwd string) (string, error) {
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("current directory: %w", err)
		}
		cwd = wd
	}
	id, err := newChatID()
	if err != nil {
		return "", err
	}
	return id, st.CreateChat(ctx, store.NewChat{ID: id, Provider: "claude", Cwd: cwd})
}

// tmuxPlace is the pane and tmux server (socket name) this process runs in,
// or empty strings outside tmux. The pane is a live pointer only: it is
// rewritten every time the chat opens, because pane ids change on restore.
func tmuxPlace() (pane, socket string) {
	pane = os.Getenv("TMUX_PANE")
	if !paneRe.MatchString(pane) {
		return "", ""
	}
	// $TMUX is "<socket path>,<server pid>,<session>"; the file name is the
	// -L name, or "default".
	path, _, _ := strings.Cut(os.Getenv("TMUX"), ",")
	if path == "" {
		return "", ""
	}
	return pane, filepath.Base(path)
}

func newChatID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("chat id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func listChats(ctx context.Context, st *store.Store, out io.Writer) error {
	chats, err := st.Chats(ctx)
	if err != nil {
		return err
	}
	for _, c := range chats {
		fmt.Fprintf(out, "%s  %-11s %s\n", c.ID, c.Status, c.Cwd)
	}
	return nil
}

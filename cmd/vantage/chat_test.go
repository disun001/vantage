package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/disun001/vantage/internal/store"
)

func TestChatRejectsBadArgs(t *testing.T) {
	t.Setenv("VANTAGE_STATE_DIR", t.TempDir())
	for _, args := range [][]string{{}, {"../etc"}, {"ABCDEF0123456789"}, {"a", "b"}} {
		if err := runChat(args, &bytes.Buffer{}); err == nil {
			t.Errorf("chat %q accepted", args)
		}
	}
}

func TestChatList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VANTAGE_STATE_DIR", dir)
	ctx := context.Background()
	st, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id, err := createChat(ctx, st, "/work/demo")
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	if !chatIDRe.MatchString(id) {
		t.Fatalf("id %q does not match chatIDRe", id)
	}
	var out bytes.Buffer
	if err := runChat([]string{"list"}, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, id) || !strings.Contains(got, "idle") || !strings.Contains(got, "/work/demo") {
		t.Errorf("list output = %q", got)
	}
}

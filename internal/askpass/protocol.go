package askpass

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// EnvAlias, EnvSocket and EnvPrompt name the variables the helper reads. The
// alias is not secret; the socket is a path only the same user can reach.
const (
	EnvAlias  = "VANTAGE_ASKPASS_ALIAS"
	EnvSocket = "VANTAGE_ASKPASS_SOCK"
	// EnvPrompt is set by ssh (8.1+) to "confirm" or "none" for prompts that
	// are not a secret.
	EnvPrompt = "SSH_ASKPASS_PROMPT"
)

const maxLine = 4096 // a request or reply is one short JSON line

type request struct {
	Alias  string `json:"alias"`
	Prompt string `json:"prompt"`
}

type reply struct {
	OK       bool   `json:"ok"`
	Password string `json:"password,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Ask sends the prompt to the server at sock. ok is false when the server
// declined to answer it (not a plain password prompt, or no stored password),
// in which case the caller falls back to the terminal. A server that cannot be
// reached is an error.
func Ask(sock, alias, prompt string) (password string, ok bool, err error) {
	c, err := net.DialTimeout("unix", sock, 2*time.Second)
	if err != nil {
		return "", false, fmt.Errorf("askpass server: %w", err)
	}
	defer func() { _ = c.Close() }() // the answer is already read
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(c).Encode(request{Alias: alias, Prompt: prompt}); err != nil {
		return "", false, fmt.Errorf("askpass request: %w", err)
	}
	var r reply
	if err := json.NewDecoder(&limited{c: c}).Decode(&r); err != nil {
		return "", false, fmt.Errorf("askpass reply: %w", err)
	}
	return r.Password, r.OK, nil
}

// limited stops a peer from making a decoder buffer without bound.
type limited struct {
	c net.Conn
	n int
}

func (l *limited) Read(p []byte) (int, error) {
	if l.n >= maxLine {
		return 0, errors.New("message too long")
	}
	n, err := l.c.Read(p[:min(len(p), maxLine-l.n)])
	l.n += n
	return n, err
}

package askpass

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/vault"
)

// Server answers prompts from an unlocked vault. Only processes of the same
// user are served, and only for a plain password prompt of the named host.
type Server struct {
	Vault vault.Vault
	Key   vault.Key
	// Host finds the ssh settings of an alias, to tell its prompt from others.
	Host func(alias string) (sshconfig.Host, error)
	// User is the local account ssh logs in as when a host sets no User.
	User string
}

// Listener is a running Server.
type Listener struct {
	// Path is the socket to put in EnvSocket.
	Path string
	ln   *net.UnixListener
	dir  string
}

// Listen starts serving on a socket in a fresh 0700 directory under base (the
// system temp dir when empty). Close stops it and removes the directory.
func (s Server) Listen(base string) (*Listener, error) {
	dir, err := os.MkdirTemp(base, "vantage-askpass-")
	if err != nil {
		return nil, fmt.Errorf("askpass dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("askpass dir: %w", err)
	}
	path := filepath.Join(dir, "s")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("askpass socket: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("askpass socket: %w", err)
	}
	l := &Listener{Path: path, ln: ln, dir: dir}
	go s.serve(ln)
	return l, nil
}

// Close stops the server and removes its socket.
func (l *Listener) Close() error {
	err := l.ln.Close()
	_ = os.RemoveAll(l.dir)
	return err
}

func (s Server) serve(ln *net.UnixListener) {
	for {
		c, err := ln.AcceptUnix()
		if err != nil {
			return // closed
		}
		go s.handle(c)
	}
}

func (s Server) handle(c *net.UnixConn) {
	defer func() { _ = c.Close() }() // one request per connection
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if uid, err := peerUID(c); err != nil || uid != uint32(os.Getuid()) { //nolint:gosec // a uid fits in 32 bits
		return
	}
	var req request
	if err := json.NewDecoder(&limited{c: c}).Decode(&req); err != nil {
		return
	}
	_ = json.NewEncoder(c).Encode(s.answer(req))
}

// answer never says why it declined beyond a short reason: the caller only
// needs to know whether to ask the terminal instead.
func (s Server) answer(req request) reply {
	h, err := s.Host(req.Alias)
	if err != nil {
		return reply{Error: "unknown host"}
	}
	if !Plain(req.Prompt, h, s.User) {
		return reply{Error: "not a password prompt for this host"}
	}
	p, err := s.Vault.Get(s.Key, req.Alias)
	if err != nil {
		if errors.Is(err, vault.ErrNoPassword) {
			return reply{Error: "no stored password"}
		}
		return reply{Error: "vault unavailable"}
	}
	return reply{OK: true, Password: p}
}

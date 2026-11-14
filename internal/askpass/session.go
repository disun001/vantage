package askpass

import (
	"sync"

	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/vault"
	"github.com/disun001/vantage/internal/workspace"
)

// Session is the unlocked vault of one running TUI. The passwords stay in this
// process's memory until it ends; the unlock server hands them to the helper of
// the user's own ssh panes.
type Session struct {
	Vault vault.Vault
	// Identity is the age identity file that opens the vault. Empty means the
	// vault has a passphrase, asked once with Unlock.
	Identity string
	// Bin is the vantage binary ssh runs as its askpass helper.
	Bin string
	// Host finds the ssh settings of an alias.
	Host func(alias string) (sshconfig.Host, error)
	// User is the local account name.
	User string
	// SocketBase is where the server's directory goes; empty means the system
	// temp dir.
	SocketBase string

	mu   sync.Mutex
	key  *vault.Key
	pass map[string]string
	ln   *Listener
}

// NeedsUnlock reports whether a vault exists that the person must still open
// with its passphrase.
func (s *Session) NeedsUnlock() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key == nil && s.Identity == "" && s.Vault.Exists()
}

// Unlock opens the vault with the passphrase (ignored for an identity vault)
// and starts the server. A wrong passphrase is vault.ErrWrongKey.
func (s *Session) Unlock(passphrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unlockLocked(passphrase)
}

func (s *Session) unlockLocked(passphrase string) error {
	if s.key != nil {
		return nil
	}
	k := vault.Key{Passphrase: passphrase, IdentityFile: s.Identity}
	all, err := s.Vault.All(k)
	if err != nil {
		return err
	}
	srv := Server{Password: s.lookup, Host: s.Host, User: s.User}
	ln, err := srv.Listen(s.SocketBase)
	if err != nil {
		return err
	}
	s.key, s.pass, s.ln = &k, all, ln
	return nil
}

// lookup serves the server from the in-memory copy.
func (s *Session) lookup(alias string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.pass[alias]; ok {
		return p, nil
	}
	return "", vault.ErrNoPassword
}

// ready opens an identity vault on first use, which needs no prompt.
func (s *Session) ready() bool {
	if s.key == nil && s.Identity != "" {
		_ = s.unlockLocked("") // an unreadable identity just means no stored passwords
	}
	return s.key != nil
}

// Askpass returns what a pane needs to answer alias's password prompt, or nil
// when the vault is locked or holds no password for it.
func (s *Session) Askpass(alias string) *workspace.Askpass {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return nil
	}
	if _, ok := s.pass[alias]; !ok {
		return nil
	}
	return &workspace.Askpass{Bin: s.Bin, Socket: s.ln.Path}
}

// State is what the vault knows about a host.
type State int

// The states a host's password can be in.
const (
	// None: no vault, or no password for the host.
	None State = iota
	// Stored: the vault is open and has a password for the host.
	Stored
	// Locked: a vault exists but is not open, so nothing is known yet.
	Locked
)

// Stored reports the state of alias's password.
func (s *Session) Stored(alias string) State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		if s.Identity == "" && s.Vault.Exists() {
			return Locked
		}
		return None
	}
	if _, ok := s.pass[alias]; ok {
		return Stored
	}
	return None
}

// Close stops the server and forgets the passwords.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		_ = s.ln.Close()
	}
	s.key, s.pass, s.ln = nil, nil, nil
}

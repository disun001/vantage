package askpass

import (
	"errors"
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
	// SocketDir, when set, is the well-known directory where the server also
	// listens, so panes started outside this TUI find it. When another TUI
	// already holds it, this one keeps a private socket only.
	SocketDir string

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

// NeedsCreate reports a passphrase vault that does not exist yet: the person
// must choose its passphrase before the first password can be stored.
func (s *Session) NeedsCreate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key == nil && s.Identity == "" && !s.Vault.Exists()
}

// Start opens an identity vault right away, so its socket is up for panes
// started outside the TUI. A passphrase vault waits for Unlock.
func (s *Session) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready()
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
	var ln *Listener
	if s.SocketDir != "" {
		ln, _ = srv.ListenAt(s.SocketDir) // in use or unusable: fall back to a private socket
	}
	if ln == nil {
		ln, err = srv.Listen(s.SocketBase)
		if err != nil {
			return err
		}
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

// Set stores the password for alias in the vault file, then in memory. The
// vault must be open.
func (s *Session) Set(alias, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return ErrLocked
	}
	if err := s.Vault.Set(*s.key, alias, password); err != nil {
		return err
	}
	s.pass[alias] = password
	return nil
}

// closedOK is what a change does when the vault is not open: nothing to change
// when there is no vault file at all, ErrLocked when there is one.
func (s *Session) closedOK() error {
	if s.Identity == "" && !s.Vault.Exists() {
		return nil
	}
	return ErrLocked
}

// Rename moves the stored password of from to to, so a renamed host keeps it.
func (s *Session) Rename(from, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return s.closedOK()
	}
	p, ok := s.pass[from]
	if !ok {
		return nil
	}
	if err := s.Vault.Rename(*s.key, from, to); err != nil {
		return err
	}
	delete(s.pass, from)
	s.pass[to] = p
	return nil
}

// Delete removes the stored password of alias. The vault must be open, unless
// there is no vault.
func (s *Session) Delete(alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return s.closedOK()
	}
	if err := s.Vault.Delete(*s.key, alias); err != nil {
		return err
	}
	delete(s.pass, alias)
	return nil
}

// ErrLocked is returned when a change needs the vault open first.
var ErrLocked = errors.New("the password vault is locked")

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

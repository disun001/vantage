package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/disun001/vantage/internal/askpass"
	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/vault"
	"github.com/disun001/vantage/internal/workspace"
)

// identityEnv names an age identity file that opens the password vault without
// a passphrase. Without it the vault asks for a passphrase.
const identityEnv = "VANTAGE_AGE_IDENTITY"

// passwords is where stored ssh passwords live and who may read them: the
// vault is passwords.age in the config dir, hosts come from the ssh config.
type passwords struct {
	vault    vault.Vault
	identity string
	bin      string // this binary, which ssh runs as its askpass helper
	user     string
	sshPath  string
	sockDir  string // where a running TUI advertises its unlock socket
}

// newPasswords reads the setup from the environment. sshPath is the ssh config.
func newPasswords(sshPath string) (passwords, error) {
	dir, err := workspace.ConfigDir()
	if err != nil {
		return passwords{}, fmt.Errorf("config dir: %w", err)
	}
	bin, err := os.Executable()
	if err != nil {
		return passwords{}, fmt.Errorf("vantage binary: %w", err)
	}
	local := ""
	if u, err := user.Current(); err == nil {
		local = u.Username
	}
	return passwords{
		vault:    vault.Vault{Path: filepath.Join(dir, "passwords.age")},
		identity: os.Getenv(identityEnv),
		bin:      bin,
		user:     local,
		sshPath:  sshPath,
		sockDir:  askpassDir(),
	}, nil
}

// askpassDir is the well-known place of the unlock socket: the user's runtime
// dir when there is one, else a per-user directory in the temp dir.
func askpassDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "vantage")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("vantage-%d", os.Getuid()))
}

// defaultSSHPath is ~/.ssh/config.
func defaultSSHPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func (p passwords) host(alias string) (sshconfig.Host, error) {
	hosts, err := sshconfig.Load(p.sshPath)
	if err != nil {
		return sshconfig.Host{}, err
	}
	for _, h := range hosts {
		if h.Alias == alias {
			return h, nil
		}
	}
	return sshconfig.Host{}, fmt.Errorf("host %q is not in the ssh config", alias)
}

// session is the unlocked vault of a running TUI.
func (p passwords) session() *askpass.Session {
	return &askpass.Session{Vault: p.vault, Identity: p.identity, Bin: p.bin, User: p.user, Host: p.host, SocketDir: p.sockDir}
}

// direct is the helper's own way into the vault, for panes no TUI serves.
func (p passwords) direct() *askpass.Direct {
	return &askpass.Direct{Vault: p.vault, Identity: p.identity, User: p.user, Host: p.host}
}

// askpassFor is the helper settings for an ssh pane about to start. This TUI's
// own session answers for the hosts it knows a password for. Else a TUI that
// advertises its socket is asked whether the host has one (no password: plain
// ssh, nothing changes for that host). With no TUI to ask and a vault on disk,
// the helper opens the vault itself, since the password cannot be known without
// the passphrase. No vault, no helper: plain ssh.
func (p passwords) askpassFor(s *askpass.Session) func(alias string) *workspace.Askpass {
	return func(alias string) *workspace.Askpass {
		if s != nil {
			if a := s.Askpass(alias); a != nil {
				return a
			}
		}
		if sock, ok := askpass.Advertised(p.sockDir); ok {
			if has, err := askpass.Has(sock, alias); err == nil {
				if !has {
					return nil
				}
				return &workspace.Askpass{Bin: p.bin, Socket: sock}
			}
		}
		if p.vault.Exists() {
			return &workspace.Askpass{Bin: p.bin}
		}
		return nil
	}
}

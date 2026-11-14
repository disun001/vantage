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
// a passphrase. Without it the vault asks for a passphrase once per run.
const identityEnv = "VANTAGE_AGE_IDENTITY"

// passwordSession builds the session of stored ssh passwords: the vault is
// passwords.age in the config dir, hosts come from the ssh config at sshPath.
func passwordSession(sshPath string) (*askpass.Session, error) {
	dir, err := workspace.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	bin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("vantage binary: %w", err)
	}
	local := ""
	if u, err := user.Current(); err == nil {
		local = u.Username
	}
	return &askpass.Session{
		Vault:    vault.Vault{Path: filepath.Join(dir, "passwords.age")},
		Identity: os.Getenv(identityEnv),
		Bin:      bin,
		User:     local,
		Host: func(alias string) (sshconfig.Host, error) {
			hosts, err := sshconfig.Load(sshPath)
			if err != nil {
				return sshconfig.Host{}, err
			}
			for _, h := range hosts {
				if h.Alias == alias {
					return h, nil
				}
			}
			return sshconfig.Host{}, fmt.Errorf("host %q is not in the ssh config", alias)
		},
	}, nil
}

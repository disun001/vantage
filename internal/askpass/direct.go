package askpass

import (
	"errors"
	"strings"

	"github.com/disun001/vantage/internal/sshconfig"
	"github.com/disun001/vantage/internal/vault"
)

// Direct answers a password prompt by opening the vault in the helper itself,
// for ssh panes that no running TUI serves (restored panes, the S / V picker).
// It asks for the vault passphrase in the pane, or uses an identity file.
type Direct struct {
	Vault vault.Vault
	// Identity is an age identity file that opens the vault without a prompt.
	Identity string
	// Host finds the ssh settings of an alias.
	Host func(alias string) (sshconfig.Host, error)
	// User is the local account name.
	User string
}

// Answer returns the stored password for alias when prompt is its plain
// password prompt. ok is false when there is nothing to answer with, so the
// caller asks the person instead; an empty passphrase says "I will type it".
func (d Direct) Answer(alias, prompt string, term Terminal) (password string, ok bool) {
	if !d.Vault.Exists() {
		return "", false
	}
	h, err := d.Host(alias)
	if err != nil || !Plain(prompt, h, d.User) {
		return "", false
	}
	key := vault.Key{IdentityFile: d.Identity}
	if d.Identity == "" {
		term.Show("vantage vault passphrase for " + alias + " (enter to type the ssh password instead): ")
		p, err := term.ReadSecret()
		term.Show("\n")
		if err != nil || strings.TrimSpace(p) == "" {
			return "", false
		}
		key.Passphrase = p
	}
	pw, err := d.Vault.Get(key, alias)
	switch {
	case errors.Is(err, vault.ErrWrongKey):
		term.Show("wrong vault passphrase\n")
		return "", false
	case err != nil:
		return "", false // no stored password: the person types it
	}
	return pw, true
}

// Package askpass answers ssh's password prompts from the password vault. ssh
// runs the helper once per prompt through SSH_ASKPASS; the helper asks a server
// that lives inside the running TUI, which holds the unlocked vault, so the
// secret never touches a command line, shell history or the environment.
// Everything the helper cannot answer from the vault goes to the terminal.
package askpass

import (
	"fmt"
	"strings"

	"github.com/disun001/vantage/internal/sshconfig"
)

// Plain reports whether prompt is ssh asking for the login password of h, and
// nothing else. Host key questions, key passphrases, verification codes and the
// password of a jump host all fail it, so the vault is never read out for them.
// localUser is the account ssh uses when the host sets no User.
func Plain(prompt string, h sshconfig.Host, localUser string) bool {
	user, host := h.User, h.HostName
	if user == "" {
		user = localUser
	}
	if host == "" {
		host = h.Alias
	}
	who := user + "@" + host
	p := strings.TrimSpace(prompt)
	return p == fmt.Sprintf("%s's password:", who) || strings.EqualFold(p, fmt.Sprintf("(%s) Password:", who))
}

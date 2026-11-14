package askpass

import (
	"path/filepath"
)

// Advertised returns the well-known socket in dir when a TUI of this user
// serves it: the directory must be private and the socket must answer.
func Advertised(dir string) (string, bool) {
	if TrustedDir(dir) != nil {
		return "", false
	}
	path := filepath.Join(dir, SocketName)
	if _, err := Has(path, "-"); err != nil {
		return "", false
	}
	return path, true
}

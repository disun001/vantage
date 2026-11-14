// Package vault keeps ssh passwords, keyed by host alias, in one file encrypted
// with age. The file is safe to keep in a public dotfiles repo; what unlocks it
// is a passphrase or an age identity file, neither of which is stored here. It
// never runs ssh and never prints a secret.
package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// ErrWrongKey means the file exists but the passphrase or identity cannot open it.
var ErrWrongKey = errors.New("wrong passphrase or identity for the password vault")

// ErrNoPassword means the vault has no password for the alias.
var ErrNoPassword = errors.New("no stored password for the host")

// Key unlocks the vault. Exactly one of the fields is set: Passphrase for a
// vault protected by a passphrase, IdentityFile (the path of an age identity
// file) for one encrypted to that identity.
type Key struct {
	Passphrase   string
	IdentityFile string
}

// Vault is the encrypted file at Path.
type Vault struct {
	Path string
	// WorkFactor is the scrypt cost for passphrase vaults. Zero means age's
	// default, which takes about a second; tests lower it.
	WorkFactor int
}

// document is what sits inside the encryption.
type document struct {
	Version   int               `json:"version"`
	Passwords map[string]string `json:"passwords"`
}

// Exists reports whether the vault file is there. A symlink counts: stow links
// a dotfiles checkout into the config dir.
func (v Vault) Exists() bool {
	_, err := os.Stat(v.Path)
	return err == nil
}

// Get returns the password stored for alias.
func (v Vault) Get(k Key, alias string) (string, error) {
	doc, err := v.read(k)
	if err != nil {
		return "", err
	}
	p, ok := doc.Passwords[alias]
	if !ok {
		return "", fmt.Errorf("%q: %w", alias, ErrNoPassword)
	}
	return p, nil
}

// All returns every stored password by alias, for a caller that keeps them in
// memory for the length of a session.
func (v Vault) All(k Key) (map[string]string, error) {
	doc, err := v.read(k)
	if err != nil {
		return nil, err
	}
	return doc.Passwords, nil
}

// Aliases returns the hosts that have a stored password.
func (v Vault) Aliases(k Key) ([]string, error) {
	doc, err := v.read(k)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(doc.Passwords))
	for a := range doc.Passwords {
		out = append(out, a)
	}
	return out, nil
}

// Set stores the password for alias, creating the vault when it is missing.
func (v Vault) Set(k Key, alias, password string) error {
	if alias == "" || password == "" {
		return errors.New("alias and password are required")
	}
	return v.change(k, func(d *document) error {
		d.Passwords[alias] = password
		return nil
	})
}

// Delete removes the password for alias. A missing one is not an error.
func (v Vault) Delete(k Key, alias string) error {
	return v.change(k, func(d *document) error {
		delete(d.Passwords, alias)
		return nil
	})
}

// Rename moves the password stored for from to to. Nothing is stored for from:
// nothing happens.
func (v Vault) Rename(k Key, from, to string) error {
	return v.change(k, func(d *document) error {
		p, ok := d.Passwords[from]
		if !ok {
			return nil
		}
		if _, taken := d.Passwords[to]; taken {
			return fmt.Errorf("%q already has a stored password", to)
		}
		d.Passwords[to] = p
		delete(d.Passwords, from)
		return nil
	})
}

// change reads the vault, applies f and writes the result back.
func (v Vault) change(k Key, f func(*document) error) error {
	doc, err := v.read(k)
	if err != nil {
		return err
	}
	if err := f(&doc); err != nil {
		return err
	}
	return v.write(k, doc)
}

// read decrypts the file. A missing file is an empty vault.
func (v Vault) read(k Key) (document, error) {
	empty := document{Version: 1, Passwords: map[string]string{}}
	target, err := v.target()
	if err != nil {
		return empty, err
	}
	raw, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		if err := k.check(); err != nil {
			return empty, err
		}
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("read vault: %w", err)
	}
	ids, err := k.identities()
	if err != nil {
		return empty, err
	}
	r, err := age.Decrypt(bytes.NewReader(raw), ids...)
	if err != nil {
		return empty, fmt.Errorf("%w: %w", ErrWrongKey, err)
	}
	var doc document
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return empty, fmt.Errorf("vault content is not readable: %w", err)
	}
	if doc.Passwords == nil {
		doc.Passwords = map[string]string{}
	}
	return doc, nil
}

// write encrypts doc and replaces the file atomically, mode 0600.
func (v Vault) write(k Key, doc document) (err error) {
	target, err := v.target()
	if err != nil {
		return err
	}
	rec, err := k.recipient(v.WorkFactor)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, rec)
	if err != nil {
		return fmt.Errorf("encrypt vault: %w", err)
	}
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		return fmt.Errorf("encode vault: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("encrypt vault: %w", err)
	}
	return replace(target, buf.Bytes())
}

// target is the file to replace. A symlink is followed, so a dotfiles link
// survives and the real file is the one rewritten.
func (v Vault) target() (string, error) {
	t, err := filepath.EvalSymlinks(v.Path)
	if err == nil {
		return t, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("resolve vault path: %w", err)
	}
	if _, lerr := os.Lstat(v.Path); lerr == nil {
		return "", errors.New("vault is a dangling symlink")
	}
	return v.Path, nil
}

func replace(target string, data []byte) (err error) {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".vault-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name()) // nothing was renamed, so nothing is lost
		}
	}()
	if err = tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err = io.Copy(tmp, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err = os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("replace vault: %w", err)
	}
	return nil
}

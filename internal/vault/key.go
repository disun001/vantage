package vault

import (
	"errors"
	"fmt"
	"os"

	"filippo.io/age"
)

// check reports whether the key names exactly one way to unlock.
func (k Key) check() error {
	switch {
	case k.Passphrase != "" && k.IdentityFile != "":
		return errors.New("use either a passphrase or an identity file, not both")
	case k.Passphrase == "" && k.IdentityFile == "":
		return errors.New("the vault needs a passphrase or an identity file")
	}
	return nil
}

// identities is what decrypts the file.
func (k Key) identities() ([]age.Identity, error) {
	if err := k.check(); err != nil {
		return nil, err
	}
	if k.Passphrase != "" {
		id, err := age.NewScryptIdentity(k.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("passphrase: %w", err)
		}
		return []age.Identity{id}, nil
	}
	return k.fileIdentities()
}

func (k Key) fileIdentities() ([]age.Identity, error) {
	f, err := os.Open(k.IdentityFile)
	if err != nil {
		return nil, fmt.Errorf("open identity file: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only: a close error loses nothing
	ids, err := age.ParseIdentities(f)
	if err != nil {
		return nil, fmt.Errorf("identity file: %w", err)
	}
	return ids, nil
}

// recipient is what encrypts the file: the passphrase itself, or the public
// half of the identity.
func (k Key) recipient(workFactor int) (age.Recipient, error) {
	if err := k.check(); err != nil {
		return nil, err
	}
	if k.Passphrase != "" {
		r, err := age.NewScryptRecipient(k.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("passphrase: %w", err)
		}
		if workFactor > 0 {
			r.SetWorkFactor(workFactor)
		}
		return r, nil
	}
	ids, err := k.fileIdentities()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		switch x := id.(type) {
		case *age.X25519Identity:
			return x.Recipient(), nil
		case *age.HybridIdentity:
			return x.Recipient(), nil
		}
	}
	return nil, errors.New("identity file holds no key vantage can encrypt to")
}

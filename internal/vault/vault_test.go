package vault

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"filippo.io/age"
)

const fast = 2 // scrypt cost for tests

func pass(p string) Key { return Key{Passphrase: p} }

func newVault(t *testing.T) Vault {
	t.Helper()
	return Vault{Path: filepath.Join(t.TempDir(), "passwords.age"), WorkFactor: fast}
}

func TestSetGetRoundTripWithPassphrase(t *testing.T) {
	v := newVault(t)
	if err := v.Set(pass("pw"), "web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(pass("pw"), "web")
	if err != nil || got != "s3cret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
}

func TestFileHoldsNoPlaintextAndIsPrivate(t *testing.T) {
	v := newVault(t)
	if err := v.Set(pass("pw"), "web", "s3cret-value"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(v.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret-value") || strings.Contains(string(raw), "web") {
		t.Error("the vault file leaks its content")
	}
	if st, err := os.Stat(v.Path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, %v", st.Mode().Perm(), err)
	}
}

func TestWrongPassphraseIsRefusedAndFileUntouched(t *testing.T) {
	v := newVault(t)
	if err := v.Set(pass("right"), "web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(v.Path)
	if _, err := v.Get(pass("wrong"), "web"); !errors.Is(err, ErrWrongKey) {
		t.Errorf("Get with a wrong passphrase: %v", err)
	}
	if err := v.Set(pass("wrong"), "db", "x"); !errors.Is(err, ErrWrongKey) {
		t.Errorf("Set with a wrong passphrase: %v", err)
	}
	if after, _ := os.ReadFile(v.Path); string(after) != string(before) {
		t.Error("a refused write changed the file")
	}
}

func TestMissingVaultIsEmptyButNeedsAKey(t *testing.T) {
	v := newVault(t)
	if v.Exists() {
		t.Error("Exists on a missing file")
	}
	if _, err := v.Get(pass("pw"), "web"); !errors.Is(err, ErrNoPassword) {
		t.Errorf("Get on a missing vault: %v", err)
	}
	if err := v.Set(Key{}, "web", "x"); err == nil {
		t.Error("Set without any key must fail")
	}
	if err := v.Set(Key{Passphrase: "a", IdentityFile: "b"}, "web", "x"); err == nil {
		t.Error("Set with two keys must fail")
	}
}

func TestDeleteAndRename(t *testing.T) {
	v := newVault(t)
	k := pass("pw")
	for a, p := range map[string]string{"web": "1", "db": "2"} {
		if err := v.Set(k, a, p); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.Rename(k, "web", "web2"); err != nil {
		t.Fatal(err)
	}
	if err := v.Rename(k, "nothing", "x"); err != nil {
		t.Errorf("renaming a host without a password: %v", err)
	}
	if err := v.Rename(k, "web2", "db"); err == nil {
		t.Error("renaming onto a stored alias must fail")
	}
	if err := v.Delete(k, "db"); err != nil {
		t.Fatal(err)
	}
	if err := v.Delete(k, "db"); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
	got, err := v.Aliases(k)
	slices.Sort(got)
	if err != nil || !reflect.DeepEqual(got, []string{"web2"}) {
		t.Errorf("Aliases = %v, %v", got, err)
	}
	if p, _ := v.Get(k, "web2"); p != "1" {
		t.Errorf("renamed password = %q", p)
	}
}

func TestIdentityFileKey(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(idPath, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := newVault(t)
	k := Key{IdentityFile: idPath}
	if err := v.Set(k, "web", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(k, "web"); err != nil || got != "s3cret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if _, err := v.Get(pass("pw"), "web"); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a passphrase must not open an identity vault: %v", err)
	}
	if err := v.Set(Key{IdentityFile: filepath.Join(t.TempDir(), "none")}, "x", "y"); err == nil {
		t.Error("a missing identity file must fail")
	}
}

func TestSymlinkedVaultStaysALink(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "dotfiles", "passwords.age")
	link := filepath.Join(dir, "config", "passwords.age")
	if err := os.MkdirAll(filepath.Dir(actual), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	v := Vault{Path: actual, WorkFactor: fast}
	if err := v.Set(pass("pw"), "web", "1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	via := Vault{Path: link, WorkFactor: fast}
	if err := via.Set(pass("pw"), "db", "2"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was replaced by a file: %v, %v", st.Mode(), err)
	}
	if got, err := v.Get(pass("pw"), "db"); err != nil || got != "2" {
		t.Errorf("actual file lacks the change: %q, %v", got, err)
	}
}

func TestEmptyInputIsRefused(t *testing.T) {
	v := newVault(t)
	if err := v.Set(pass("pw"), "", "x"); err == nil {
		t.Error("empty alias accepted")
	}
	if err := v.Set(pass("pw"), "web", ""); err == nil {
		t.Error("empty password accepted")
	}
}

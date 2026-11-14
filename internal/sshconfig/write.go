package sshconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/disun001/vantage/internal/workspace"
)

// File is an ssh config file that vantage can read and change.
type File struct{ Path string }

// BackupSuffix is added to the path for the copy taken before each write.
const BackupSuffix = ".vantage.bak"

// List returns the concrete hosts of the file.
func (f File) List() ([]Host, error) { return Load(f.Path) }

// Own returns the options the alias's own Host block sets, without wildcard
// defaults. It is what an edit starts from.
func (f File) Own(alias string) (Host, error) {
	content, _, err := f.read()
	if err != nil {
		return Host{}, err
	}
	lines, err := splitLines(content)
	if err != nil {
		return Host{}, err
	}
	return ownValues(lines, alias)
}

// Add appends a Host block for h. The alias must not exist yet.
func (f File) Add(h Host) error {
	if err := Validate(h); err != nil {
		return err
	}
	return f.write(h.Alias, &h, func(old string) (string, error) { return addBlock(old, h) })
}

// Edit sets HostName, User, Port, ProxyJump and IdentityFile of the block
// named from. Setting an identity also sets IdentitiesOnly yes, and clearing it
// removes that option again.
// Options that already match are left alone and an empty value removes the
// option. When h.Alias differs from from the block is renamed too, and the
// ProxyJump hops of other blocks that name the old alias follow. It is refused
// when the new alias exists or a jump through the old one cannot be rewritten
// (it lives in an included file).
func (f File) Edit(from string, h Host) error {
	if err := Validate(h); err != nil {
		return err
	}
	if !workspace.ValidSSHAlias(from) {
		return fmt.Errorf("alias %q is not a plain host alias", from)
	}
	return f.write(from, &h, func(old string) (string, error) { return editBlock(old, from, h) })
}

// Delete removes the Host block named by alias alone. Comments above the
// block stay; a block that shares its Host line with other patterns is
// refused.
func (f File) Delete(alias string) error {
	if !workspace.ValidSSHAlias(alias) {
		return fmt.Errorf("alias %q is not a plain host alias", alias)
	}
	return f.write(alias, nil, func(old string) (string, error) { return deleteBlock(old, alias) })
}

// read returns the file text and mode. A missing file reads as empty.
func (f File) read() (string, fs.FileMode, error) {
	target, err := f.target()
	if err != nil {
		return "", 0, err
	}
	b, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		return "", 0o600, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("read ssh config: %w", err)
	}
	st, err := os.Stat(target)
	if err != nil {
		return "", 0, fmt.Errorf("stat ssh config: %w", err)
	}
	return string(b), st.Mode().Perm(), nil
}

// target is the file to replace. A symlinked config (a dotfiles checkout) is
// followed, so the link survives and the real file is the one rewritten.
func (f File) target() (string, error) {
	t, err := filepath.EvalSymlinks(f.Path)
	if err == nil {
		return t, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("resolve ssh config: %w", err)
	}
	if _, lerr := os.Lstat(f.Path); lerr == nil {
		return "", errors.New("ssh config is a dangling symlink")
	}
	return f.Path, nil
}

// write runs one guarded change to alias's block. The file is only replaced
// when
//   - it parses before and after the change,
//   - no host other than alias (and the name it is renamed to) changed, and
//     the block holds exactly the options in want (nil: the block is gone),
//   - the file was not modified by someone else meanwhile.
//
// The old content is copied to Path+BackupSuffix first, and the new content
// reaches the file by an atomic rename of a temp file in the same directory.
func (f File) write(from string, want *Host, change func(old string) (string, error)) error {
	target, err := f.target()
	if err != nil {
		return err
	}
	old, mode, err := f.read()
	if err != nil {
		return err
	}
	before, err := Hosts(strings.NewReader(old))
	if err != nil {
		return fmt.Errorf("refusing to edit: %w", err)
	}
	next, err := change(old)
	if err != nil {
		return err
	}
	if next == old {
		return nil // already as asked: leave the file and the backup alone
	}
	if err := verify(before, next, from, want); err != nil {
		return fmt.Errorf("refusing to write: %w", err)
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if old != "" {
		if err := os.WriteFile(f.Path+BackupSuffix, []byte(old), 0o600); err != nil {
			return fmt.Errorf("backup ssh config: %w", err)
		}
	}
	return replace(target, old, next, mode)
}

// verify re-parses the new text and compares it with the old. from is the
// block that changes; want.Alias is the name it ends up with.
func verify(before []Host, next, from string, want *Host) error {
	to := from
	if want != nil {
		to = want.Alias
	}
	after, err := Hosts(strings.NewReader(next))
	if err != nil {
		return fmt.Errorf("new config does not parse: %w", err)
	}
	// A rename also repoints the hops that went through the old alias; nothing
	// else about the other hosts may differ.
	expect := withJumpsRenamed(before, from, to)
	if !reflect.DeepEqual(without(expect, from, to), without(after, from, to)) {
		if users := jumpUsers(after, from); to != from && len(users) > 0 {
			return fmt.Errorf("%w: %s (set where vantage cannot change it, such as an Include)", ErrReferenced, strings.Join(users, ", "))
		}
		return errors.New("the change would alter other hosts")
	}
	lines, err := splitLines(next)
	if err != nil {
		return err
	}
	if _, err := ownValues(lines, from); want == nil || to != from { // the old name must be gone
		if !errors.Is(err, ErrNotFound) {
			return errors.New("the host would still be in the file")
		}
	}
	if want == nil {
		return nil
	}
	got, err := ownValues(lines, to)
	if err != nil {
		return fmt.Errorf("new config lacks the host: %w", err)
	}
	if got != *want {
		return fmt.Errorf("new config has %+v, wanted %+v", got, *want)
	}
	return nil
}

func without(hs []Host, aliases ...string) []Host {
	var out []Host
	for _, h := range hs {
		if !slices.Contains(aliases, h.Alias) {
			out = append(out, h)
		}
	}
	return out
}

// jumpUsers names the hosts whose ProxyJump goes through alias, as the
// effective values say (a wildcard default counts). At most five are named.
func jumpUsers(hs []Host, alias string) []string {
	var out []string
	for _, h := range hs {
		if h.Alias != alias && jumpsThrough(h.ProxyJump, alias) {
			out = append(out, h.Alias)
		}
	}
	if len(out) > 5 {
		out = append(out[:5], "…")
	}
	return out
}

// jumpsThrough reports whether a ProxyJump list has a hop that names alias.
func jumpsThrough(list, alias string) bool {
	for _, hop := range strings.Split(list, ",") {
		if _, host, _ := splitHop(hop); host == alias {
			return true
		}
	}
	return false
}

// splitHop cuts one ProxyJump hop, [ssh://][user@]host[:port], into what
// comes before the host, the host, and what follows it.
func splitHop(hop string) (pre, host, post string) {
	if strings.HasPrefix(hop, "ssh://") {
		pre, hop = "ssh://", hop[len("ssh://"):]
	}
	if i := strings.LastIndex(hop, "@"); i >= 0 {
		pre, hop = pre+hop[:i+1], hop[i+1:]
	}
	if i := strings.LastIndex(hop, ":"); i >= 0 && !strings.HasSuffix(hop, "]") {
		hop, post = hop[:i], hop[i:]
	}
	return pre, hop, post
}

// renameJump returns list with every hop that names from pointing at to
// instead. User and port of a hop stay.
func renameJump(list, from, to string) string {
	hops := strings.Split(list, ",")
	for i, hop := range hops {
		if pre, host, post := splitHop(hop); host == from {
			hops[i] = pre + to + post
		}
	}
	return strings.Join(hops, ",")
}

// withJumpsRenamed copies hs with from renamed to to in every ProxyJump.
func withJumpsRenamed(hs []Host, from, to string) []Host {
	out := append([]Host(nil), hs...)
	for i := range out {
		out[i].ProxyJump = renameJump(out[i].ProxyJump, from, to)
	}
	return out
}

// replace swaps in next atomically, unless target no longer holds old.
func replace(target, old, next string, mode fs.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".config.vantage-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name()) // nothing was renamed, so nothing is lost
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err = tmp.WriteString(next); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	cur, rerr := os.ReadFile(target)
	if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
		return fmt.Errorf("recheck ssh config: %w", rerr)
	}
	if !bytes.Equal(cur, []byte(old)) {
		return errors.New("ssh config changed while editing; nothing written")
	}
	if err = os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("replace ssh config: %w", err)
	}
	return nil
}

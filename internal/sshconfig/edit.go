package sshconfig

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/disun001/vantage/internal/workspace"
)

// Errors a caller can tell apart.
var (
	// ErrNotFound means no Host block in the file is named by the alias alone.
	ErrNotFound = errors.New("host is not in the ssh config file")
	// ErrShared means the alias shares its Host line with other patterns, so
	// editing the block would change those hosts too.
	ErrShared = errors.New("host shares a Host line with other patterns; edit the file by hand")
	// ErrExists means the alias is already a Host pattern.
	ErrExists = errors.New("host already exists")
	// ErrReferenced means other hosts jump through the alias in a place that
	// cannot be rewritten, so renaming it would break them.
	ErrReferenced = errors.New("other hosts jump through it")
)

// The editable options, in the order a new block lists them.
var fields = []struct {
	key string
	get func(Host) string
}{
	{"HostName", func(h Host) string { return h.HostName }},
	{"User", func(h Host) string { return h.User }},
	{"Port", func(h Host) string { return h.Port }},
	{"ProxyJump", func(h Host) string { return h.ProxyJump }},
	{"IdentityFile", func(h Host) string { return h.Identity }},
}

// identitiesOnly goes with IdentityFile: it keeps ssh from offering every key
// the agent holds, which can end in "too many authentication failures".
const identitiesOnly = "IdentitiesOnly"

// Values go on a line ssh reads token by token and may end up on a command
// line, so each is held to the characters its option can really contain: no
// whitespace, quotes, '#', '$' or '%' escapes, and no leading '-'.
var (
	hostNameRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$|^\[[0-9A-Fa-f:.]+\]$`)
	userRe      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
	proxyJumpRe = regexp.MustCompile(`^[A-Za-z0-9_@\[\]:.,-]+$`)
	// identityRe takes a path with an optional leading ~ and no spaces: ssh
	// splits the line on whitespace and expands % and $ tokens in it.
	identityRe = regexp.MustCompile(`^[A-Za-z0-9_~/.][A-Za-z0-9_~/.@+=-]*$`)
)

// Validate reports whether h can be written: the alias must be a plain host
// alias and every set option must pass its own check. Empty options are fine.
func Validate(h Host) error {
	if !workspace.ValidSSHAlias(h.Alias) {
		return fmt.Errorf("alias %q: use letters, digits and . _ @ %% + - only", h.Alias)
	}
	if h.HostName != "" && !hostNameRe.MatchString(h.HostName) {
		return fmt.Errorf("host %q is not a host name or address", h.HostName)
	}
	if h.User != "" && !userRe.MatchString(h.User) {
		return fmt.Errorf("user %q has characters ssh would misread", h.User)
	}
	if h.Port != "" {
		if n, err := strconv.Atoi(h.Port); err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != h.Port {
			return fmt.Errorf("port %q is not 1-65535", h.Port)
		}
	}
	if h.ProxyJump != "" && (h.ProxyJump[0] == '-' || !proxyJumpRe.MatchString(h.ProxyJump)) {
		return fmt.Errorf("jump %q is not a list of [user@]host[:port]", h.ProxyJump)
	}
	if h.Identity != "" && !identityRe.MatchString(h.Identity) {
		return fmt.Errorf("identity %q: use a path without spaces or quotes", h.Identity)
	}
	return nil
}

// line is one parsed config line.
type line struct {
	key    string // option or section keyword, "" for blank and comment lines
	value  string // everything after the keyword, trimmed
	prefix string // text before the value, kept so a replaced line looks the same
}

func parseLine(s string) line {
	t := strings.TrimLeft(s, " \t")
	if t == "" || t[0] == '#' {
		return line{}
	}
	end := strings.IndexAny(t, " \t=")
	if end < 0 {
		return line{key: t}
	}
	rest := t[end:]
	val := strings.TrimLeft(rest, " \t")
	if strings.HasPrefix(val, "=") {
		val = strings.TrimLeft(val[1:], " \t")
	}
	val = strings.TrimRight(val, " \t")
	cut := strings.LastIndex(s, val)
	if val == "" {
		cut = len(s)
	}
	return line{key: t[:end], value: val, prefix: s[:cut]}
}

func is(l line, key string) bool { return strings.EqualFold(l.key, key) }

// block is a Host or Match section: the header line at lines[start] and the
// lines up to end (exclusive).
type block struct {
	start, end int
	patterns   []string // the Host line's patterns
	match      bool     // a Match section: never editable, ends the Host blocks before it
}

// blocks lists the sections of a config file. Quoted patterns are rare and
// hard to compare safely, so a Host line holding a quote gets one pattern that
// no alias can equal.
func blocks(lines []string) []block {
	var out []block
	for i, s := range lines {
		l := parseLine(s)
		switch {
		case is(l, "Host"):
			ps := strings.Fields(l.value)
			if strings.ContainsAny(l.value, `"'`) {
				ps = []string{"\x00quoted"}
			}
			out = append(out, block{start: i, patterns: ps})
		case is(l, "Match"):
			out = append(out, block{start: i, match: true})
		default:
			continue
		}
		if n := len(out); n > 1 {
			out[n-2].end = i
		}
	}
	if n := len(out); n > 0 {
		out[n-1].end = len(lines)
	}
	return out
}

func hasAlias(b block, alias string) bool {
	for _, p := range b.patterns {
		if p == alias {
			return true
		}
	}
	return false
}

// own finds the Host block named by alias alone.
func own(lines []string, alias string) (block, error) {
	for _, b := range blocks(lines) {
		if !hasAlias(b, alias) {
			continue
		}
		if len(b.patterns) != 1 {
			return b, ErrShared
		}
		return b, nil
	}
	return block{}, ErrNotFound
}

// ownValues reads the options as the alias's own block sets them,
// ignoring wildcard defaults: that is what an edit changes.
func ownValues(lines []string, alias string) (Host, error) {
	b, err := own(lines, alias)
	if err != nil {
		return Host{}, err
	}
	h := Host{Alias: alias}
	set := map[string]*string{
		"hostname": &h.HostName, "user": &h.User, "port": &h.Port, "proxyjump": &h.ProxyJump,
		"identityfile": &h.Identity,
	}
	seen := map[string]bool{}
	for _, s := range lines[b.start+1 : b.end] {
		l := parseLine(s)
		k := strings.ToLower(l.key)
		if p, ok := set[k]; ok && !seen[k] {
			*p, seen[k] = l.value, true // ssh keeps the first value it sees
		}
	}
	// A key path often carries a trailing comment; it is not part of the path.
	if i := strings.Index(h.Identity, " #"); i >= 0 {
		h.Identity = strings.TrimSpace(h.Identity[:i])
	}
	return h, nil
}

// splitLines turns file text into lines. A file with CR characters is
// refused: rewriting it would change every line ending.
func splitLines(content string) ([]string, error) {
	if strings.Contains(content, "\r") {
		return nil, errors.New("ssh config has CRLF line endings; edit it by hand")
	}
	if content == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n"), nil
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// editBlock sets the options of from's block and, when h.Alias differs,
// renames the block and repoints the ProxyJump hops of other blocks. Only options that differ from the block's own values are
// touched; empty clears the option.
func editBlock(content, from string, h Host) (string, error) {
	lines, err := splitLines(content)
	if err != nil {
		return "", err
	}
	cur, err := ownValues(lines, from)
	if err != nil {
		return "", err
	}
	if from != h.Alias {
		for _, b := range blocks(lines) {
			if hasAlias(b, h.Alias) {
				return "", fmt.Errorf("%q: %w", h.Alias, ErrExists)
			}
		}
	}
	if from != h.Alias {
		b, _ := own(lines, from)
		renameJumps(lines, b, from, h.Alias)
	}
	for _, f := range fields {
		if f.get(cur) == f.get(h) {
			continue
		}
		b, _ := own(lines, from) // found above; lines moved since
		lines = setOption(lines, b, f.key, f.get(h))
	}
	if cur.Identity != h.Identity {
		b, _ := own(lines, from)
		value := "" // clearing the identity clears the option it brought
		if h.Identity != "" {
			value = "yes"
		}
		lines = setOption(lines, b, identitiesOnly, value)
	}
	if from != h.Alias {
		b, _ := own(lines, from)
		lines[b.start] = parseLine(lines[b.start]).prefix + h.Alias
	}
	return joinLines(lines), nil
}

// renameJumps repoints, in place, every ProxyJump line outside the skip block
// whose hops name from. Lines keep their layout; only the hop text changes.
func renameJumps(lines []string, skip block, from, to string) {
	for i, s := range lines {
		if i >= skip.start && i < skip.end {
			continue
		}
		l := parseLine(s)
		if !is(l, "ProxyJump") || !jumpsThrough(l.value, from) {
			continue
		}
		lines[i] = l.prefix + renameJump(l.value, from, to)
	}
}

// setOption replaces, removes or appends one option inside block b. A new
// option goes after the block's last option, indented like the first one.
func setOption(lines []string, b block, key, value string) []string {
	indent, last := "  ", b.start
	firstSeen := false
	for i := b.start + 1; i < b.end; i++ {
		if parseLine(lines[i]).key == "" {
			continue
		}
		if !firstSeen {
			indent = lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
			firstSeen = true
		}
		last = i
	}
	var out []string
	found := false
	for i, s := range lines {
		l := parseLine(s)
		if i > b.start && i < b.end && is(l, key) {
			if value == "" {
				continue // clearing removes every copy
			}
			if !found {
				s = l.prefix + value
			}
			found = true
		}
		out = append(out, s)
		if i == last && value != "" && !hasOption(lines, b, key) {
			out = append(out, indent+key+" "+value)
		}
	}
	return out
}

func hasOption(lines []string, b block, key string) bool {
	for _, s := range lines[b.start+1 : b.end] {
		if is(parseLine(s), key) {
			return true
		}
	}
	return false
}

// addBlock appends a Host block for h. It goes ahead of the first wildcard
// Host or Match section, because ssh keeps the first value it finds and
// such a section is normally a set of defaults that must come last.
func addBlock(content string, h Host) (string, error) {
	lines, err := splitLines(content)
	if err != nil {
		return "", err
	}
	bs := blocks(lines)
	at := len(lines)
	for _, b := range bs {
		if hasAlias(b, h.Alias) {
			return "", fmt.Errorf("%q: %w", h.Alias, ErrExists)
		}
	}
	for _, b := range bs {
		if b.match || strings.ContainsAny(strings.Join(b.patterns, ""), "*?!") {
			at = b.start
			break
		}
	}
	for at > 0 && strings.HasPrefix(strings.TrimSpace(lines[at-1]), "#") {
		at-- // a comment right above a section belongs to it
	}
	add := []string{"Host " + h.Alias}
	for _, f := range fields {
		if v := f.get(h); v != "" {
			add = append(add, "  "+f.key+" "+v)
		}
	}
	if h.Identity != "" {
		add = append(add, "  "+identitiesOnly+" yes")
	}
	if at > 0 && strings.TrimSpace(lines[at-1]) != "" {
		add = append([]string{""}, add...)
	}
	if at < len(lines) {
		add = append(add, "")
	}
	out := append(append(append([]string{}, lines[:at]...), add...), lines[at:]...)
	return joinLines(out), nil
}

// deleteBlock removes the Host block named by alias alone. Comment lines
// directly above the next section are kept: they belong to it.
func deleteBlock(content, alias string) (string, error) {
	lines, err := splitLines(content)
	if err != nil {
		return "", err
	}
	b, err := own(lines, alias)
	if err != nil {
		return "", err
	}
	end := b.end
	if end < len(lines) {
		for end > b.start+1 && strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#") {
			end--
		}
	}
	out := append(append([]string{}, lines[:b.start]...), lines[end:]...)
	if end == len(lines) { // the block was last: no blank line should trail it
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
	}
	return joinLines(out), nil
}

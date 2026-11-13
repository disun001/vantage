// Package project defines the project definition file (one TOML per project:
// a set of git repos), plus loading, validation, and the layout of its clones
// on disk. It runs git only through internal/git and never touches tmux.
package project

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// Project is a named set of git repos managed together.
type Project struct {
	Name string `toml:"name"`
	// Flat puts the single repo directly at the project path (and, for
	// tasks, at the task path) instead of one directory below it. Needs
	// exactly one repo and no dir.
	Flat  bool   `toml:"flat"`
	Repos []Repo `toml:"repo"`
}

// Repo is one git repository of a project.
type Repo struct {
	URL string `toml:"url"`
	// Dir is the clone's directory name under the project dir. Defaults to
	// the last element of URL without ".git".
	Dir string `toml:"dir"`
	// Copy lists untracked files (globs allowed, relative to the repo) that
	// task worktrees need, such as .env. Used by tasks, not by sync.
	Copy []string `toml:"copy"`
}

var (
	// Same charset as workspace names, so a project name is safe as a file
	// name, a directory name and part of a tmux session name.
	nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	// One path element, not an option, no separators.
	dirRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// scp-like ssh syntax: user@host:path.
	scpRe = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[^\s]+$`)
	// Transports we accept. ext:: (runs commands) and anything exotic is out.
	schemeRe = regexp.MustCompile(`^(https?|ssh|git|file)://[^\s]+$`)
)

// DirName is the clone's directory name: Dir, else derived from URL.
func (r Repo) DirName() string {
	if r.Dir != "" {
		return r.Dir
	}
	u := strings.TrimRight(r.URL, "/")
	if i := strings.LastIndexAny(u, "/:"); i >= 0 {
		u = u[i+1:]
	}
	return strings.TrimSuffix(u, ".git")
}

// validURL reports whether url is safe to hand to `git clone`: a known
// transport, scp-like ssh, or an absolute local path; never an option, never
// whitespace or control characters.
func validURL(url string) bool {
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsAny(url, "\x00\r\n\t ") {
		return false
	}
	return schemeRe.MatchString(url) || scpRe.MatchString(url) || filepath.IsAbs(url)
}

// Validate checks structural rules. Everything here reaches git or the file
// system, so it is strict (AGENTS.md rule 3).
func (p Project) Validate() error {
	if !nameRe.MatchString(p.Name) {
		return fmt.Errorf("invalid project name %q (use letters, digits, '-', '_')", p.Name)
	}
	if len(p.Repos) == 0 {
		return fmt.Errorf("project %q has no repos", p.Name)
	}
	if p.Flat && (len(p.Repos) != 1 || p.Repos[0].Dir != "") {
		return fmt.Errorf("project %q: flat needs exactly one repo and no dir", p.Name)
	}
	seen := map[string]int{}
	for i, r := range p.Repos {
		where := fmt.Sprintf("repo %d", i+1)
		if !validURL(r.URL) {
			return fmt.Errorf("%s: invalid url %q", where, r.URL)
		}
		dir := r.DirName()
		if !dirRe.MatchString(dir) || dir == ".git" {
			return fmt.Errorf("%s: invalid dir %q (set dir to a plain name)", where, dir)
		}
		if first, dup := seen[dir]; dup {
			return fmt.Errorf("%s: dir %q already used by repo %d", where, dir, first)
		}
		seen[dir] = i + 1
		for _, c := range r.Copy {
			if err := validCopy(c); err != nil {
				return fmt.Errorf("%s: copy %q: %w", where, c, err)
			}
		}
	}
	return nil
}

// validCopy accepts a relative path that stays inside the repo and does not
// reach into .git.
func validCopy(c string) error {
	clean := path.Clean(c)
	switch {
	case c == "" || strings.ContainsAny(c, "\x00\r\n"):
		return fmt.Errorf("empty or contains control characters")
	case strings.Contains(c, `\`) || !filepath.IsLocal(c):
		return fmt.Errorf("must be a relative path inside the repo")
	case clean == ".git" || strings.HasPrefix(clean, ".git/"):
		return fmt.Errorf("cannot reach into .git")
	}
	return nil
}

// parse is Load for file text. file only names the file in errors and
// supplies the default project name.
func parse(text, file string) (Project, error) {
	var p Project
	md, err := toml.Decode(text, &p)
	if err != nil {
		return p, fmt.Errorf("%s: %w", file, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		keys := make([]string, len(un))
		for i, k := range un {
			keys[i] = k.String()
		}
		return p, fmt.Errorf("%s: unknown keys: %s", file, strings.Join(keys, ", "))
	}
	if p.Name == "" {
		p.Name = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}
	if err := p.Validate(); err != nil {
		return p, fmt.Errorf("%s: %w", file, err)
	}
	return p, nil
}

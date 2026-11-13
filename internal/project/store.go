package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/disun001/vantage/internal/workspace"
)

// ErrNoRoot means the projects directory is not configured.
var ErrNoRoot = errors.New("projects directory not set: export VANTAGE_PROJECTS_DIR or set projects_dir in <config>/config.toml")

// settings is <config>/config.toml.
type settings struct {
	ProjectsDir string `toml:"projects_dir"`
}

// Dir returns the project definitions directory, <config>/projects.
func Dir() (string, error) {
	c, err := workspace.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "projects"), nil
}

// Root returns the directory holding all projects' clones:
// $VANTAGE_PROJECTS_DIR, else projects_dir in <config>/config.toml. There is
// no default, so vantage never creates repos somewhere unchosen. The result
// is absolute ("~" expands).
func Root() (string, error) {
	dir := os.Getenv("VANTAGE_PROJECTS_DIR")
	if dir == "" {
		c, err := workspace.ConfigDir()
		if err != nil {
			return "", err
		}
		dir, err = readProjectsDir(filepath.Join(c, "config.toml"))
		if err != nil {
			return "", err
		}
	}
	if dir == "" {
		return "", ErrNoRoot
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("projects directory %q must be an absolute path", dir)
	}
	return filepath.Clean(dir), nil
}

func readProjectsDir(file string) (string, error) {
	var s settings
	md, err := toml.DecodeFile(file, &s)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		return "", fmt.Errorf("%s: unknown keys: %v", file, un)
	}
	return s.ProjectsDir, nil
}

// PathOf is the definition file of the project called name. It never treats
// name as a path, so a name cannot point outside Dir.
func PathOf(name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", fmt.Errorf("invalid project name %q", name)
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".toml"), nil
}

// Load reads and validates the project called name.
func Load(name string) (Project, error) {
	file, err := PathOf(name)
	if err != nil {
		return Project{}, err
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return Project{}, fmt.Errorf("project %q not found", name)
	}
	if err != nil {
		return Project{}, err
	}
	return parse(string(data), file)
}

// List returns the names of defined projects, sorted.
func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".toml"); ok && !e.IsDir() {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Path is the project's directory under root.
func (p Project) Path(root string) string { return filepath.Join(root, p.Name) }

// RepoPath is the main clone of r under root. A flat project's clone is the
// project dir itself.
func (p Project) RepoPath(root string, r Repo) string {
	if p.Flat {
		return p.Path(root)
	}
	return filepath.Join(p.Path(root), r.DirName())
}

// saveProject mirrors Project for writing: empty optional keys are omitted so
// the file reads like one written by hand.
type saveProject struct {
	Name  string     `toml:"name"`
	Flat  bool       `toml:"flat,omitempty"`
	Repos []saveRepo `toml:"repo"`
}

type saveRepo struct {
	URL  string   `toml:"url"`
	Dir  string   `toml:"dir,omitempty"`
	Copy []string `toml:"copy,omitempty"`
}

// Save validates p and writes its definition file. It never replaces an
// existing project.
func Save(p Project) (err error) {
	if err := p.Validate(); err != nil {
		return err
	}
	file, err := PathOf(p.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("project %q already exists", p.Name)
	}
	if err != nil {
		return err
	}
	// A failed Close can mean lost data, so it is part of the result.
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	out := saveProject{Name: p.Name, Flat: p.Flat}
	for _, r := range p.Repos {
		out.Repos = append(out.Repos, saveRepo(r))
	}
	return toml.NewEncoder(f).Encode(out)
}

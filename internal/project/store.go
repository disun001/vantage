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

// RepoPath is the main clone of r under root.
func (p Project) RepoPath(root string, r Repo) string {
	return filepath.Join(p.Path(root), r.DirName())
}

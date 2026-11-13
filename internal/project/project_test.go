package project

import (
	"strings"
	"testing"
)

func TestParseFull(t *testing.T) {
	p, err := parse(`
[[repo]]
url = "git@example.invalid:org/api.git"
copy = [".env", "config/*.local.json"]

[[repo]]
url = "https://example.invalid/org/web"
dir = "frontend"
`, "/x/stack.toml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "stack" {
		t.Errorf("name defaults to file base name, got %q", p.Name)
	}
	if got := p.Repos[0].DirName(); got != "api" {
		t.Errorf("derived dir = %q", got)
	}
	if got := p.Repos[1].DirName(); got != "frontend" {
		t.Errorf("explicit dir = %q", got)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := parse("[[repo]]\nurl = \"https://example.invalid/a\"\ncpoy = []\n", "x.toml")
	if err == nil || !strings.Contains(err.Error(), "unknown keys") {
		t.Fatalf("want unknown keys error, got %v", err)
	}
}

func TestDirName(t *testing.T) {
	for url, want := range map[string]string{
		"git@example.invalid:org/api.git":   "api",
		"https://example.invalid/org/web/":  "web",
		"ssh://git@example.invalid/a/b.git": "b",
		"/srv/repos/local":                  "local",
		"user@host:solo.git":                "solo",
	} {
		if got := (Repo{URL: url}).DirName(); got != want {
			t.Errorf("DirName(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Repo{URL: "https://example.invalid/org/api.git"}
	with := func(r ...Repo) Project { return Project{Name: "p", Repos: r} }
	cases := []struct {
		name string
		p    Project
		want string // substring of the error, "" for valid
	}{
		{"valid", with(ok), ""},
		{"bad name", Project{Name: "a b", Repos: []Repo{ok}}, "invalid project name"},
		{"dotted name", Project{Name: "a.b", Repos: []Repo{ok}}, "invalid project name"},
		{"no repos", with(), "no repos"},
		{"empty url", with(Repo{Dir: "x"}), "invalid url"},
		{"option url", with(Repo{URL: "--upload-pack=evil", Dir: "x"}), "invalid url"},
		{"ext transport", with(Repo{URL: "ext::sh -c evil", Dir: "x"}), "invalid url"},
		{"space in url", with(Repo{URL: "https://example.invalid/a b", Dir: "x"}), "invalid url"},
		{"newline in url", with(Repo{URL: "https://example.invalid/a\nb", Dir: "x"}), "invalid url"},
		{"relative path url", with(Repo{URL: "../repo", Dir: "x"}), "invalid url"},
		{"abs path url", with(Repo{URL: "/srv/repo"}), ""},
		{"dir traversal", with(Repo{URL: ok.URL, Dir: "../x"}), "invalid dir"},
		{"dir slash", with(Repo{URL: ok.URL, Dir: "a/b"}), "invalid dir"},
		{"dir dotdot", with(Repo{URL: ok.URL, Dir: ".."}), "invalid dir"},
		{"dir option", with(Repo{URL: ok.URL, Dir: "-x"}), "invalid dir"},
		{"dir .git", with(Repo{URL: ok.URL, Dir: ".git"}), "invalid dir"},
		{"derived dir empty", with(Repo{URL: "ssh://example.invalid/.git"}), "invalid dir"},
		{"duplicate dir", with(ok, Repo{URL: "https://example.invalid/other/api.git"}), "already used"},
		{"copy abs", with(Repo{URL: ok.URL, Copy: []string{"/etc/passwd"}}), "relative"},
		{"copy dotdot", with(Repo{URL: ok.URL, Copy: []string{"../x"}}), "relative"},
		{"copy sneaky dotdot", with(Repo{URL: ok.URL, Copy: []string{"a/../../x"}}), "relative"},
		{"copy .git", with(Repo{URL: ok.URL, Copy: []string{".git/config"}}), ".git"},
		{"copy empty", with(Repo{URL: ok.URL, Copy: []string{""}}), "empty"},
		{"copy glob ok", with(Repo{URL: ok.URL, Copy: []string{".env*", "cfg/*.json"}}), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.p.Validate()
			switch {
			case c.want == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Errorf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

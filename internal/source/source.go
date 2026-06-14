// Package source installs and updates theme bundles from git repositories
// or local folders.
package source

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CuriousFurBytes/lumos/internal/theme"
)

// originFile records where an installed theme came from so it can be
// updated later. It lives inside each installed theme directory.
const originFile = ".lumos-origin"

// Cloner fetches and refreshes git repositories. It is an interface so the
// install/update flow can be tested without touching the network.
type Cloner interface {
	Clone(url, dest string) error
	Pull(dir string) error
}

// GitCloner shells out to the system git binary.
type GitCloner struct{}

// Clone runs `git clone --depth 1 url dest`.
func (GitCloner) Clone(url, dest string) error {
	return run("", "git", "clone", "--depth", "1", url, dest)
}

// Pull runs `git pull` inside dir.
func (GitCloner) Pull(dir string) error { return run(dir, "git", "pull", "--ff-only") }

func run(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

var ownerRepo = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

// NormalizeGitURL accepts the shorthand forms lumos supports and returns a
// URL git can clone. A bare "owner/repo" is assumed to be on GitHub.
func NormalizeGitURL(spec string) string {
	switch {
	case strings.HasPrefix(spec, "http://"),
		strings.HasPrefix(spec, "https://"),
		strings.HasPrefix(spec, "git@"),
		strings.HasPrefix(spec, "ssh://"):
		return spec
	case strings.HasPrefix(spec, "github.com/"):
		return "https://" + spec
	case ownerRepo.MatchString(spec):
		return "https://github.com/" + spec
	default:
		return spec
	}
}

// Install installs the theme(s) found in spec, which is either a local
// folder or a git repository reference. It returns the slugs installed.
// When --enable behaviour is wanted the caller enables the returned slug.
func Install(spec, themesDir string, cloner Cloner) ([]string, error) {
	var src, origin string
	if info, err := os.Stat(spec); err == nil && info.IsDir() {
		abs, err := filepath.Abs(spec)
		if err != nil {
			return nil, err
		}
		src, origin = abs, abs
	} else {
		url := NormalizeGitURL(spec)
		tmp, err := os.MkdirTemp("", "lumos-clone-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		dest := filepath.Join(tmp, "repo")
		if err := cloner.Clone(url, dest); err != nil {
			return nil, fmt.Errorf("cloning %s: %w", url, err)
		}
		src, origin = dest, url
	}

	bundles := findThemeDirs(src)
	if len(bundles) == 0 {
		return nil, fmt.Errorf("no theme.toml found in %s", spec)
	}

	var slugs []string
	for _, b := range bundles {
		th, err := theme.Load(b)
		if err != nil {
			return nil, err
		}
		dest := filepath.Join(themesDir, th.Slug)
		if err := os.RemoveAll(dest); err != nil {
			return nil, err
		}
		if err := copyTree(b, dest); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dest, originFile), []byte(origin+"\n"), 0o644); err != nil {
			return nil, err
		}
		slugs = append(slugs, th.Slug)
	}
	return slugs, nil
}

// Update refreshes one theme by slug, or all installed themes when name is
// empty, by re-fetching from each theme's recorded origin.
func Update(name, themesDir string, cloner Cloner) error {
	var slugs []string
	if name != "" {
		slugs = []string{name}
	} else {
		entries, err := os.ReadDir(themesDir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				slugs = append(slugs, e.Name())
			}
		}
	}

	for _, slug := range slugs {
		dir := filepath.Join(themesDir, slug)
		origin, err := readOrigin(dir)
		if err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
		if isLocalPath(origin) {
			if err := copyTree(origin, dir); err != nil {
				return fmt.Errorf("%s: %w", slug, err)
			}
			if err := os.WriteFile(filepath.Join(dir, originFile), []byte(origin+"\n"), 0o644); err != nil {
				return err
			}
			continue
		}
		if err := cloner.Pull(dir); err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
	}
	return nil
}

func readOrigin(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, originFile))
	if os.IsNotExist(err) {
		return "", fmt.Errorf("theme is not installed by lumos (no %s)", originFile)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func isLocalPath(origin string) bool {
	info, err := os.Stat(origin)
	return err == nil && info.IsDir()
}

// findThemeDirs locates theme bundles in root: the root itself, its direct
// children, and the children of a top-level "themes" directory.
func findThemeDirs(root string) []string {
	var found []string
	add := func(dir string) {
		if _, err := os.Stat(filepath.Join(dir, theme.Manifest)); err == nil {
			found = append(found, dir)
		}
	}
	add(root)
	for _, sub := range []string{root, filepath.Join(root, "themes")} {
		entries, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				add(filepath.Join(sub, e.Name()))
			}
		}
	}
	return dedupe(found)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// copyTree recursively copies src into dst, skipping any .git directory.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

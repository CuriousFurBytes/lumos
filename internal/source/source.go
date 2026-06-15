// Package source installs and updates theme bundles from git repositories,
// local folders or local .zip files. Installed themes are always stored as
// <slug>.zip in the themes directory.
package source

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CuriousFurBytes/lumos/internal/theme"
)

// originExt is the suffix of the sidecar recording where an installed theme
// came from. For theme "dracula" the sidecar is "dracula.origin" alongside
// "dracula.zip".
const originExt = ".origin"

// Cloner fetches a git repository into a directory. It is an interface so
// install/update can be tested without network access.
type Cloner interface {
	Clone(url, dest string) error
}

// GitCloner shells out to the system git binary.
type GitCloner struct{}

// Clone runs `git clone --depth 1 url dest`.
func (GitCloner) Clone(url, dest string) error {
	c := exec.Command("git", "clone", "--depth", "1", url, dest)
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

// Install installs the theme bundle(s) referenced by spec — a local .zip
// file, a bundle directory, a folder of bundles, or a git repository — into
// themesDir as <slug>.zip, recording each theme's origin. It returns the
// installed slugs.
func Install(spec, themesDir string, cloner Cloner) ([]string, error) {
	src, origin, cleanup, err := materialize(spec, cloner)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return installFrom(src, origin, themesDir)
}

// materialize turns spec into a local path to scan plus the origin string to
// record (the original spec/URL).
func materialize(spec string, cloner Cloner) (src, origin string, cleanup func(), err error) {
	cleanup = func() {}
	if _, statErr := os.Stat(spec); statErr == nil {
		abs, aerr := filepath.Abs(spec)
		if aerr != nil {
			return "", "", cleanup, aerr
		}
		return abs, abs, cleanup, nil
	}

	url := NormalizeGitURL(spec)
	tmp, terr := os.MkdirTemp("", "lumos-clone-")
	if terr != nil {
		return "", "", cleanup, terr
	}
	cleanup = func() { os.RemoveAll(tmp) }
	dest := filepath.Join(tmp, "repo")
	if err := cloner.Clone(url, dest); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("cloning %s: %w", url, err)
	}
	return dest, url, cleanup, nil
}

func installFrom(src, origin, themesDir string) ([]string, error) {
	bundles, err := findBundles(src)
	if err != nil {
		return nil, err
	}
	if len(bundles) == 0 {
		return nil, fmt.Errorf("no theme bundles found in %s", origin)
	}
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return nil, err
	}
	var slugs []string
	for _, b := range bundles {
		th, err := theme.Load(b)
		if err != nil {
			return nil, err
		}
		dest := filepath.Join(themesDir, th.Slug+".zip")
		if isZip(b) {
			if err := copyFile(b, dest); err != nil {
				return nil, err
			}
		} else if err := zipDir(b, dest); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(themesDir, th.Slug+originExt), []byte(origin+"\n"), 0o644); err != nil {
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
		themes, err := theme.Discover(themesDir)
		if err != nil {
			return err
		}
		for _, th := range themes {
			slugs = append(slugs, th.Slug)
		}
	}

	for _, slug := range slugs {
		origin, err := readOrigin(themesDir, slug)
		if err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
		src, _, cleanup, err := materialize(origin, cloner)
		if err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
		_, err = installFrom(src, origin, themesDir)
		cleanup()
		if err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
	}
	return nil
}

func readOrigin(themesDir, slug string) (string, error) {
	b, err := os.ReadFile(filepath.Join(themesDir, slug+originExt))
	if os.IsNotExist(err) {
		return "", fmt.Errorf("theme is not installed by lumos (no origin recorded)")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func isZip(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".zip")
}

// findBundles returns the theme bundles reachable from root: a single .zip
// file, a single bundle directory, or — for a folder/clone — every .zip and
// every directory containing a manifest found by walking the tree.
func findBundles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if isZip(root) {
			return []string{root}, nil
		}
		return nil, nil
	}
	if _, err := os.Stat(filepath.Join(root, theme.Manifest)); err == nil {
		return []string{root}, nil
	}

	var found []string
	err = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			if fi.Name() == ".git" {
				return filepath.SkipDir
			}
			if _, e := os.Stat(filepath.Join(path, theme.Manifest)); e == nil {
				found = append(found, path)
				return filepath.SkipDir // don't descend into a bundle
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".zip") {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}

// zipDir packs the bundle directory dir into a zip at dest.
func zipDir(dir, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
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
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(path, filepath.Join(dst, rel))
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

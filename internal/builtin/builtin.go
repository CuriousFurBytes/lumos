// Package builtin embeds a small set of starter themes so lumos is useful
// out of the box, and seeds them into the user's themes directory on first
// run without clobbering anything the user has edited.
package builtin

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/CuriousFurBytes/lumos/internal/theme"
)

//go:embed all:themes
var themesFS embed.FS

// Names returns the slugs of the embedded starter themes.
func Names() []string {
	entries, err := themesFS.ReadDir("themes")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Seed copies any embedded theme that is not already present in themesDir.
// It returns the slugs it newly wrote. Existing themes are left untouched
// so user edits and updates are preserved.
func Seed(themesDir string) ([]string, error) {
	var seeded []string
	for _, slug := range Names() {
		dest := filepath.Join(themesDir, slug)
		if _, err := os.Stat(filepath.Join(dest, theme.Manifest)); err == nil {
			continue // already present
		}
		if err := extract(filepath.Join("themes", slug), dest); err != nil {
			return seeded, err
		}
		seeded = append(seeded, slug)
	}
	return seeded, nil
}

func extract(embedDir, dest string) error {
	return fs.WalkDir(themesFS, embedDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(embedDir, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := themesFS.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

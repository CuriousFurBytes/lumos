// Package builtin embeds a small set of starter themes so lumos is useful
// out of the box, and seeds them into the user's themes directory on first
// run without clobbering anything the user has edited.
package builtin

import (
	"embed"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed themes/*.yaml
var themesFS embed.FS

// Names returns the slugs of the embedded starter themes.
func Names() []string {
	entries, err := themesFS.ReadDir("themes")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
	}
	sort.Strings(names)
	return names
}

// Seed writes any embedded theme not already present in themesDir, returning
// the slugs it newly wrote. Existing themes are left untouched so user edits
// and updates are preserved.
func Seed(themesDir string) ([]string, error) {
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return nil, err
	}
	entries, err := themesFS.ReadDir("themes")
	if err != nil {
		return nil, err
	}
	var seeded []string
	for _, e := range entries {
		dest := filepath.Join(themesDir, e.Name())
		if _, err := os.Stat(dest); err == nil {
			continue // already present
		}
		data, err := themesFS.ReadFile("themes/" + e.Name())
		if err != nil {
			return seeded, err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return seeded, err
		}
		seeded = append(seeded, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
	}
	return seeded, nil
}

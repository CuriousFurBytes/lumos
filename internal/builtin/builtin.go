// Package builtin embeds a small set of starter theme bundles so lumos is
// useful out of the box. On first run it packs each bundle into a
// <slug>.zip in the user's themes directory, without clobbering anything the
// user has already installed or edited.
package builtin

import (
	"archive/zip"
	"embed"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
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

// Seed writes a <slug>.zip for every embedded theme not already present in
// themesDir, returning the slugs it newly wrote.
func Seed(themesDir string) ([]string, error) {
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return nil, err
	}
	var seeded []string
	for _, slug := range Names() {
		dest := filepath.Join(themesDir, slug+".zip")
		if _, err := os.Stat(dest); err == nil {
			continue // already present
		}
		if err := writeZip(dest, path.Join("themes", slug)); err != nil {
			return seeded, err
		}
		seeded = append(seeded, slug)
	}
	return seeded, nil
}

// writeZip packs the embedded bundle rooted at embedDir into a zip at dest,
// stripping the embed prefix so entries are relative to the bundle root.
func writeZip(dest, embedDir string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = fs.WalkDir(themesFS, embedDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(embedDir, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := themesFS.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

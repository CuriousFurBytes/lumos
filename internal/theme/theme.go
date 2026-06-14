// Package theme models lumos theme bundles and loads them from TOML.
//
// A theme bundle is a directory containing a theme.toml manifest plus the
// colour files it references. The manifest lists the programs the theme
// touches and, for each, the file to install and where it should go.
package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Program describes how a theme is applied to a single program/port.
type Program struct {
	// Name is the port key, e.g. "alacritty". It is matched against the
	// embedded registry to fill in a default Target when none is given.
	Name string `toml:"name"`
	// Target is the destination path for the theme file. It may contain
	// the placeholders understood by config.Paths.Expand. When empty the
	// registry default for Name is used.
	Target string `toml:"target"`
	// File is the theme file inside the bundle to install, relative to the
	// bundle directory.
	File string `toml:"file"`
	// Description is an optional human note shown in verbose output.
	Description string `toml:"description"`
	// Post lists shell commands to run after the file is written, e.g.
	// "bat cache --build", used to reload or rebuild caches.
	Post []string `toml:"post"`
}

// Theme is a parsed theme bundle.
type Theme struct {
	Name        string    `toml:"name"`
	Slug        string    `toml:"slug"`
	Family      string    `toml:"family"`
	Flavor      string    `toml:"flavor"`
	Source      string    `toml:"source"`
	Description string    `toml:"description"`
	Programs    []Program `toml:"programs"`
	// Dir is the absolute bundle directory; not present in the TOML.
	Dir string `toml:"-"`
}

// Manifest is the name of the per-theme manifest file.
const Manifest = "theme.toml"

// Load reads the theme bundle rooted at dir.
func Load(dir string) (Theme, error) {
	var th Theme
	if _, err := toml.DecodeFile(filepath.Join(dir, Manifest), &th); err != nil {
		return Theme{}, fmt.Errorf("loading %s: %w", dir, err)
	}
	th.Dir = dir
	if th.Slug == "" {
		th.Slug = filepath.Base(dir)
	}
	if err := th.validate(); err != nil {
		return Theme{}, fmt.Errorf("%s: %w", th.Slug, err)
	}
	return th, nil
}

func (t Theme) validate() error {
	if t.Name == "" {
		return fmt.Errorf("missing required field 'name'")
	}
	if len(t.Programs) == 0 {
		return fmt.Errorf("theme defines no programs")
	}
	for i, p := range t.Programs {
		if p.Name == "" {
			return fmt.Errorf("programs[%d] missing 'name'", i)
		}
	}
	return nil
}

// Discover loads every theme bundle directly under themesDir. Directories
// without a manifest are skipped. A missing themesDir yields no themes and
// no error. Results are sorted by slug for stable display.
func Discover(themesDir string) ([]Theme, error) {
	entries, err := os.ReadDir(themesDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var themes []Theme
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(themesDir, e.Name())
		if _, err := os.Stat(filepath.Join(dir, Manifest)); err != nil {
			continue
		}
		th, err := Load(dir)
		if err != nil {
			return nil, err
		}
		themes = append(themes, th)
	}
	sort.Slice(themes, func(i, j int) bool { return themes[i].Slug < themes[j].Slug })
	return themes, nil
}

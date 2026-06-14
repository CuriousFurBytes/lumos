// Package theme models lumos theme files and loads them from YAML.
//
// A theme is a single YAML 1.2 document describing one or more variants
// (e.g. light/dark flavours). Programs are declared once at the theme level
// as templates that reference the active variant's colour palette, so a
// single file can theme every supported program for any variant.
package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Program declares how a theme renders for a single program/port.
type Program struct {
	// Name is the port key, e.g. "alacritty"; matched against the registry
	// to fill in a default Target and post-hooks.
	Name string `yaml:"name"`
	// Target overrides the registry default destination path.
	Target string `yaml:"target"`
	// Template is rendered with the variant palette: ${color.KEY} tokens are
	// replaced by the variant's colors[KEY].
	Template string `yaml:"template"`
	// Content is literal output used as-is, for programs that do not depend
	// on the palette. Exactly one of Template or Content must be set.
	Content string `yaml:"content"`
	// Post lists reload/rebuild hooks to run after writing.
	Post []string `yaml:"post"`
}

// Variant is one flavour of a theme with its own palette.
type Variant struct {
	ID     string            `yaml:"id"`
	Name   string            `yaml:"name"`
	Style  string            `yaml:"style"` // e.g. "light" or "dark"
	Colors map[string]string `yaml:"colors"`
}

// Theme is a parsed theme file.
type Theme struct {
	Name        string    `yaml:"name"`
	Slug        string    `yaml:"slug"`
	Family      string    `yaml:"family"`
	Source      string    `yaml:"source"`
	Description string    `yaml:"description"`
	Programs    []Program `yaml:"programs"`
	Variants    []Variant `yaml:"variants"`
	// Path is the absolute file path; not present in the YAML.
	Path string `yaml:"-"`
}

// Extensions recognised as theme files.
var Extensions = []string{".yaml", ".yml"}

// Load reads and validates the theme file at path.
func Load(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		return Theme{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	th.Path = path
	if th.Slug == "" {
		th.Slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	for i := range th.Variants {
		if th.Variants[i].ID == "" {
			th.Variants[i].ID = Slugify(th.Variants[i].Name)
		}
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
	if len(t.Variants) == 0 {
		return fmt.Errorf("theme defines no variants")
	}
	for i, p := range t.Programs {
		if p.Name == "" {
			return fmt.Errorf("programs[%d] missing 'name'", i)
		}
		if p.Template == "" && p.Content == "" {
			return fmt.Errorf("program %q: needs a 'template' or 'content'", p.Name)
		}
	}
	for i, v := range t.Variants {
		if v.ID == "" {
			return fmt.Errorf("variants[%d] missing 'id' or 'name'", i)
		}
		if v.Name == "" {
			return fmt.Errorf("variant %q missing 'name'", v.ID)
		}
	}
	return nil
}

// Variant returns the variant with the given id.
func (t Theme) Variant(id string) (Variant, bool) {
	for _, v := range t.Variants {
		if v.ID == id {
			return v, true
		}
	}
	return Variant{}, false
}

// DefaultVariant returns the first variant.
func (t Theme) DefaultVariant() Variant { return t.Variants[0] }

// Discover loads every theme file directly under dir, sorted by slug. A
// missing dir yields no themes and no error.
func Discover(dir string) ([]Theme, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var themes []Theme
	for _, e := range entries {
		if e.IsDir() || !HasThemeExt(e.Name()) {
			continue
		}
		th, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		themes = append(themes, th)
	}
	sort.Slice(themes, func(i, j int) bool { return themes[i].Slug < themes[j].Slug })
	return themes, nil
}

// HasThemeExt reports whether name has a recognised theme extension.
func HasThemeExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range Extensions {
		if ext == e {
			return true
		}
	}
	return false
}

// Slugify lowercases s and replaces spaces/underscores with hyphens.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Join(strings.Fields(s), "-")
}

// Package theme models lumos theme bundles and loads them from a zip
// archive (the distributable form) or a directory (handy for authoring).
//
// A bundle contains a theme.yaml manifest with the theme's metadata and
// variants (each a named palette), plus a programs/ directory in which each
// file is the config/template for one program. The manifest does not list
// programs — they are discovered from the files, and each file's name (minus
// extension) is the registry port key. Files may use ${color.KEY} tokens,
// filled from the active variant's palette at apply time.
package theme

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest is the per-bundle metadata file.
const Manifest = "theme.yaml"

// ProgramsDir is the bundle subdirectory holding per-program config files.
const ProgramsDir = "programs"

// Program is one program's config template, discovered from a bundle file.
type Program struct {
	// Port is the registry key, taken from the file name without extension.
	Port string
	// Template is the file body; ${color.KEY} tokens are filled per variant.
	Template string
}

// Variant is one flavour of a theme with its own palette.
type Variant struct {
	ID     string            `yaml:"id"`
	Name   string            `yaml:"name"`
	Style  string            `yaml:"style"`
	Colors map[string]string `yaml:"colors"`
}

// Theme is a loaded bundle.
type Theme struct {
	Name        string
	Slug        string
	Family      string
	Source      string
	Description string
	Variants    []Variant
	Programs    []Program
	// Path is the bundle's zip file or directory.
	Path string
}

// manifest is the YAML schema of theme.yaml — metadata and variants only.
type manifest struct {
	Name        string    `yaml:"name"`
	Slug        string    `yaml:"slug"`
	Family      string    `yaml:"family"`
	Source      string    `yaml:"source"`
	Description string    `yaml:"description"`
	Variants    []Variant `yaml:"variants"`
}

// Load reads the bundle at path, which may be a .zip file or a directory.
func Load(path string) (Theme, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Theme{}, err
	}
	if info.IsDir() {
		return loadFS(os.DirFS(path), filepath.Base(path), path)
	}
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return Theme{}, fmt.Errorf("%s: not a .zip bundle or directory", path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Theme{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer zr.Close()
	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return loadFS(zr, slug, path)
}

func loadFS(fsys fs.FS, slugDefault, srcPath string) (Theme, error) {
	data, err := fs.ReadFile(fsys, Manifest)
	if err != nil {
		return Theme{}, fmt.Errorf("%s: reading %s: %w", srcPath, Manifest, err)
	}
	var m manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Theme{}, fmt.Errorf("%s: parsing %s: %w", srcPath, Manifest, err)
	}

	programs, err := readPrograms(fsys)
	if err != nil {
		return Theme{}, fmt.Errorf("%s: %w", srcPath, err)
	}

	th := Theme{
		Name:        m.Name,
		Slug:        m.Slug,
		Family:      m.Family,
		Source:      m.Source,
		Description: m.Description,
		Variants:    m.Variants,
		Programs:    programs,
		Path:        srcPath,
	}
	if th.Slug == "" {
		th.Slug = slugDefault
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

func readPrograms(fsys fs.FS) ([]Program, error) {
	entries, err := fs.ReadDir(fsys, ProgramsDir)
	if errIsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var programs []Program
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := fs.ReadFile(fsys, path.Join(ProgramsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		programs = append(programs, Program{
			Port:     portFromFile(e.Name()),
			Template: string(body),
		})
	}
	sort.Slice(programs, func(i, j int) bool { return programs[i].Port < programs[j].Port })
	return programs, nil
}

func errIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(err.Error(), "file does not exist"))
}

// portFromFile maps a program file name to its registry port key by dropping
// the extension, e.g. "alacritty.toml" -> "alacritty".
func portFromFile(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func (t Theme) validate() error {
	if t.Name == "" {
		return fmt.Errorf("missing required field 'name'")
	}
	if len(t.Variants) == 0 {
		return fmt.Errorf("theme defines no variants")
	}
	if len(t.Programs) == 0 {
		return fmt.Errorf("theme has no program files under %s/", ProgramsDir)
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

// Program returns the program for the given port.
func (t Theme) Program(port string) (Program, bool) {
	for _, p := range t.Programs {
		if p.Port == port {
			return p, true
		}
	}
	return Program{}, false
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

// Discover loads every theme bundle directly under dir: each .zip file, and
// each subdirectory containing a manifest. Results are sorted by slug. A
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
		p := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
			if _, err := os.Stat(filepath.Join(p, Manifest)); err != nil {
				continue
			}
		case strings.EqualFold(filepath.Ext(e.Name()), ".zip"):
			// load below
		default:
			continue
		}
		th, err := Load(p)
		if err != nil {
			return nil, err
		}
		themes = append(themes, th)
	}
	sort.Slice(themes, func(i, j int) bool { return themes[i].Slug < themes[j].Slug })
	return themes, nil
}

// Slugify lowercases s and replaces spaces/underscores with hyphens.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Join(strings.Fields(s), "-")
}

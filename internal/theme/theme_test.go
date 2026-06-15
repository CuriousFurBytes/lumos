package theme

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// writeBundle writes an unpacked theme bundle (a directory) and returns its
// path. Program files live under programs/.
func writeBundle(t *testing.T, parent, slug, manifest string, programs map[string]string) string {
	t.Helper()
	dir := filepath.Join(parent, slug)
	if err := os.MkdirAll(filepath.Join(dir, "programs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range programs {
		if err := os.WriteFile(filepath.Join(dir, "programs", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// zipDir packs a bundle directory into <slug>.zip and returns the zip path.
func zipDir(t *testing.T, dir, slug, dest string) string {
	t.Helper()
	zpath := filepath.Join(dest, slug+".zip")
	f, err := os.Create(zpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, _ := os.ReadFile(p)
		_, err = w.Write(data)
		return err
	})
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zpath
}

const catppuccinManifest = `
name: "Catppuccin"
family: catppuccin
source: "https://github.com/catppuccin/catppuccin"
variants:
  - id: latte
    name: "Latte"
    style: light
    colors: { base: "#eff1f5", text: "#4c4f69" }
  - id: mocha
    name: "Mocha"
    style: dark
    colors: { base: "#1e1e2e", text: "#cdd6f4" }
`

func catppuccinPrograms() map[string]string {
	return map[string]string{
		"alacritty.toml": "bg = \"${color.base}\"\nfg = \"${color.text}\"",
		"kitty.conf":     "background ${color.base}",
	}
}

func TestLoadDirectory(t *testing.T) {
	dir := writeBundle(t, t.TempDir(), "catppuccin", catppuccinManifest, catppuccinPrograms())
	th, err := Load(dir)
	if err != nil {
		t.Fatalf("Load dir: %v", err)
	}
	if th.Name != "Catppuccin" || th.Slug != "catppuccin" {
		t.Errorf("name/slug = %q/%q", th.Name, th.Slug)
	}
	if len(th.Variants) != 2 {
		t.Fatalf("variants = %d", len(th.Variants))
	}
	if len(th.Programs) != 2 {
		t.Fatalf("programs = %d, want 2", len(th.Programs))
	}
	// Programs are discovered from the files; the port key is the file stem.
	got := map[string]bool{}
	for _, p := range th.Programs {
		got[p.Port] = true
	}
	if !got["alacritty"] || !got["kitty"] {
		t.Errorf("ports = %v, want alacritty+kitty", got)
	}
}

func TestLoadZip(t *testing.T) {
	tmp := t.TempDir()
	dir := writeBundle(t, tmp, "catppuccin", catppuccinManifest, catppuccinPrograms())
	zpath := zipDir(t, dir, "catppuccin", tmp)

	th, err := Load(zpath)
	if err != nil {
		t.Fatalf("Load zip: %v", err)
	}
	if th.Slug != "catppuccin" {
		t.Errorf("slug = %q", th.Slug)
	}
	if len(th.Programs) != 2 {
		t.Errorf("programs = %d, want 2", len(th.Programs))
	}
	// Program template body must be readable from the zip.
	for _, p := range th.Programs {
		if p.Port == "alacritty" && p.Template == "" {
			t.Error("alacritty template empty")
		}
	}
}

func TestProgramLookup(t *testing.T) {
	dir := writeBundle(t, t.TempDir(), "catppuccin", catppuccinManifest, catppuccinPrograms())
	th, _ := Load(dir)
	if _, ok := th.Program("kitty"); !ok {
		t.Error("kitty not found")
	}
	if _, ok := th.Program("nope"); ok {
		t.Error("unexpected program")
	}
}

func TestVariantLookupAndDefault(t *testing.T) {
	dir := writeBundle(t, t.TempDir(), "catppuccin", catppuccinManifest, catppuccinPrograms())
	th, _ := Load(dir)
	if v, ok := th.Variant("mocha"); !ok || v.Name != "Mocha" {
		t.Errorf("Variant(mocha) = %+v,%v", v, ok)
	}
	if th.DefaultVariant().ID != "latte" {
		t.Errorf("default = %q, want latte", th.DefaultVariant().ID)
	}
}

func TestVariantIDDefaultsFromName(t *testing.T) {
	m := "name: Solo\nvariants:\n  - name: \"Soft Dark\"\n    colors: {}\n"
	dir := writeBundle(t, t.TempDir(), "solo", m, map[string]string{"vim.vim": "x"})
	th, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if th.Variants[0].ID != "soft-dark" {
		t.Errorf("derived id = %q", th.Variants[0].ID)
	}
}

func TestLoadValidation(t *testing.T) {
	parent := t.TempDir()
	progs := map[string]string{"vim.vim": "x"}
	cases := map[string]struct {
		manifest string
		programs map[string]string
	}{
		"no name":     {"variants:\n  - {id: a, name: A}\n", progs},
		"no variants": {"name: X\n", progs},
		"no programs": {"name: X\nvariants:\n  - {id: a, name: A}\n", map[string]string{}},
	}
	for label, c := range cases {
		dir := writeBundle(t, parent, "bad-"+label, c.manifest, c.programs)
		if _, err := Load(dir); err == nil {
			t.Errorf("%s: expected validation error", label)
		}
	}
}

func TestDiscoverFindsZipsAndDirs(t *testing.T) {
	dir := t.TempDir()
	// a zip theme
	b := writeBundle(t, t.TempDir(), "zeta", catppuccinManifest, catppuccinPrograms())
	zipDir(t, b, "zeta", dir)
	// a directory theme
	writeBundle(t, dir, "alpha", catppuccinManifest, catppuccinPrograms())
	// noise
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)

	themes, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("got %d themes, want 2", len(themes))
	}
	if themes[0].Slug != "alpha" || themes[1].Slug != "zeta" {
		t.Errorf("order = %q,%q want alpha,zeta", themes[0].Slug, themes[1].Slug)
	}
}

func TestDiscoverMissingDir(t *testing.T) {
	themes, err := Discover(filepath.Join(t.TempDir(), "nope"))
	if err != nil || len(themes) != 0 {
		t.Errorf("got %d,%v want 0,nil", len(themes), err)
	}
}

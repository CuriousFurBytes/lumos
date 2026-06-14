package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTheme(t *testing.T, dir, file, body string) string {
	t.Helper()
	p := filepath.Join(dir, file)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const catppuccinYAML = `
name: "Catppuccin"
family: catppuccin
source: "https://github.com/catppuccin/catppuccin"
programs:
  - name: alacritty
    template: |
      [colors.primary]
      background = "${color.base}"
      foreground = "${color.text}"
  - name: bat
    content: "static bat theme"
    post: ["bat cache --build"]
variants:
  - id: latte
    name: "Latte"
    style: light
    colors:
      base: "#eff1f5"
      text: "#4c4f69"
  - id: mocha
    name: "Mocha"
    style: dark
    colors:
      base: "#1e1e2e"
      text: "#cdd6f4"
`

func TestLoadParsesVariants(t *testing.T) {
	dir := t.TempDir()
	p := writeTheme(t, dir, "catppuccin.yaml", catppuccinYAML)

	th, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if th.Name != "Catppuccin" {
		t.Errorf("Name = %q", th.Name)
	}
	if th.Slug != "catppuccin" {
		t.Errorf("Slug = %q, want catppuccin (from filename)", th.Slug)
	}
	if len(th.Variants) != 2 {
		t.Fatalf("variants = %d, want 2", len(th.Variants))
	}
	if th.Variants[0].ID != "latte" || th.Variants[0].Style != "light" {
		t.Errorf("variant[0] = %+v", th.Variants[0])
	}
	if len(th.Programs) != 2 {
		t.Fatalf("programs = %d, want 2", len(th.Programs))
	}
	if th.Path != p {
		t.Errorf("Path = %q, want %q", th.Path, p)
	}
}

func TestVariantLookupAndDefault(t *testing.T) {
	dir := t.TempDir()
	th, err := Load(writeTheme(t, dir, "catppuccin.yaml", catppuccinYAML))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := th.Variant("mocha")
	if !ok || v.Name != "Mocha" {
		t.Errorf("Variant(mocha) = %+v, %v", v, ok)
	}
	if _, ok := th.Variant("nope"); ok {
		t.Error("unexpected variant")
	}
	if th.DefaultVariant().ID != "latte" {
		t.Errorf("DefaultVariant = %q, want latte (first)", th.DefaultVariant().ID)
	}
}

func TestVariantIDDefaultsFromName(t *testing.T) {
	dir := t.TempDir()
	body := `
name: "Solo"
programs:
  - name: vim
    content: "x"
variants:
  - name: "Soft Dark"
    style: dark
    colors: {}
`
	th, err := Load(writeTheme(t, dir, "solo.yaml", body))
	if err != nil {
		t.Fatal(err)
	}
	if th.Variants[0].ID != "soft-dark" {
		t.Errorf("derived id = %q, want soft-dark", th.Variants[0].ID)
	}
}

func TestLoadValidation(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"no name":      "programs:\n  - name: vim\n    content: x\nvariants:\n  - {id: a, name: A}\n",
		"no programs":  "name: X\nvariants:\n  - {id: a, name: A}\n",
		"no variants":  "name: X\nprograms:\n  - {name: vim, content: x}\n",
		"prog no name": "name: X\nprograms:\n  - {content: x}\nvariants:\n  - {id: a, name: A}\n",
		"prog empty":   "name: X\nprograms:\n  - {name: vim}\nvariants:\n  - {id: a, name: A}\n",
	}
	for label, body := range cases {
		if _, err := Load(writeTheme(t, dir, "bad.yaml", body)); err == nil {
			t.Errorf("%s: expected validation error", label)
		}
	}
}

func TestDiscoverScansYAML(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "zzz.yaml", "name: Z\nprograms:\n  - {name: vim, content: x}\nvariants:\n  - {id: a, name: A}\n")
	writeTheme(t, dir, "aaa.yml", "name: A\nprograms:\n  - {name: vim, content: x}\nvariants:\n  - {id: a, name: A}\n")
	writeTheme(t, dir, "notes.txt", "ignore me")

	themes, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("got %d themes, want 2", len(themes))
	}
	if themes[0].Slug != "aaa" || themes[1].Slug != "zzz" {
		t.Errorf("order = %q,%q want aaa,zzz", themes[0].Slug, themes[1].Slug)
	}
}

func TestDiscoverMissingDir(t *testing.T) {
	themes, err := Discover(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("Discover missing dir: %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("got %d, want 0", len(themes))
	}
}

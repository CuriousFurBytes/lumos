package builtin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/theme"
)

func TestNamesIncludesStarterThemes(t *testing.T) {
	names := Names()
	if len(names) < 2 {
		t.Fatalf("expected several builtin themes, got %v", names)
	}
	want := map[string]bool{"catppuccin": false, "dracula": false, "rose-pine": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("builtin theme %q missing", n)
		}
	}
}

func TestEmbeddedThemesParseAndRender(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	themes, err := theme.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(themes) == 0 {
		t.Fatal("no themes discovered after seed")
	}
	// Catppuccin should ship multiple variants; every program template must
	// resolve against every variant palette.
	for _, th := range themes {
		if len(th.Variants) == 0 {
			t.Errorf("%s has no variants", th.Slug)
		}
	}
}

func TestCatppuccinHasFourVariants(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(filepath.Join(dir, "catppuccin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(th.Variants) != 4 {
		t.Errorf("catppuccin variants = %d, want 4", len(th.Variants))
	}
}

func TestSeedSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "dracula.yaml")
	if err := os.WriteFile(existing, []byte("name: edited\nprograms:\n  - {name: vim, content: x}\nvariants:\n  - {id: a, name: A}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seeded, err := Seed(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range seeded {
		if s == "dracula" {
			t.Fatal("Seed overwrote an existing theme")
		}
	}
	data, _ := os.ReadFile(existing)
	if string(data) != "name: edited\nprograms:\n  - {name: vim, content: x}\nvariants:\n  - {id: a, name: A}\n" {
		t.Error("existing theme content changed")
	}
}

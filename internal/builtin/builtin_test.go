package builtin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/theme"
)

func TestNamesIncludesStarterThemes(t *testing.T) {
	names := Names()
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

func TestSeedProducesLoadableZips(t *testing.T) {
	dir := t.TempDir()
	seeded, err := Seed(dir)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(seeded) == 0 {
		t.Fatal("nothing seeded")
	}
	// Every seeded bundle must be a real .zip that loads and has variants.
	themes, err := theme.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(themes) != len(seeded) {
		t.Errorf("discovered %d, seeded %d", len(themes), len(seeded))
	}
	for _, th := range themes {
		if len(th.Variants) == 0 {
			t.Errorf("%s has no variants", th.Slug)
		}
		if len(th.Programs) == 0 {
			t.Errorf("%s has no programs", th.Slug)
		}
	}
}

func TestCatppuccinHasFourVariants(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(filepath.Join(dir, "catppuccin.zip"))
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
	existing := filepath.Join(dir, "dracula.zip")
	if err := os.WriteFile(existing, []byte("user data"), 0o644); err != nil {
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
	if string(data) != "user data" {
		t.Error("existing theme content changed")
	}
}

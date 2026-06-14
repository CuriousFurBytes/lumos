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
	want := map[string]bool{"catppuccin-mocha": false, "dracula": false}
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

func TestSeedWritesThemesAndTheyParse(t *testing.T) {
	dir := t.TempDir()
	seeded, err := Seed(dir)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(seeded) == 0 {
		t.Fatal("nothing seeded")
	}
	themes, err := theme.Discover(dir)
	if err != nil {
		t.Fatalf("Discover after seed: %v", err)
	}
	if len(themes) != len(seeded) {
		t.Errorf("discovered %d themes, seeded %d", len(themes), len(seeded))
	}
}

func TestSeedSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	// Mark one theme as user-modified; a second seed must not overwrite it.
	existing := filepath.Join(dir, "dracula", "theme.toml")
	if err := os.WriteFile(existing, []byte("name=\"edited\"\n[[programs]]\nname=\"vim\"\nfile=\"x\"\n"), 0o644); err != nil {
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
	if string(data) != "name=\"edited\"\n[[programs]]\nname=\"vim\"\nfile=\"x\"\n" {
		t.Error("existing theme content changed")
	}
}

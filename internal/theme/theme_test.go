package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTheme(t *testing.T, dir, name, body string) string {
	t.Helper()
	td := filepath.Join(dir, name)
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "theme.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return td
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	td := writeTheme(t, dir, "catppuccin-mocha", `
name = "Catppuccin Mocha"
family = "catppuccin"
flavor = "mocha"
source = "https://github.com/catppuccin/catppuccin"

[[programs]]
name = "alacritty"
file = "alacritty.toml"

[[programs]]
name = "bat"
target = "~/.config/bat/themes/mocha.tmTheme"
file = "bat.tmTheme"
post = ["bat cache --build"]
`)

	got, err := Load(td)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != "Catppuccin Mocha" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Slug != "catppuccin-mocha" {
		t.Errorf("Slug = %q, want catppuccin-mocha (from dir)", got.Slug)
	}
	if got.Family != "catppuccin" {
		t.Errorf("Family = %q", got.Family)
	}
	if len(got.Programs) != 2 {
		t.Fatalf("Programs = %d, want 2", len(got.Programs))
	}
	if got.Programs[1].Post[0] != "bat cache --build" {
		t.Errorf("post = %v", got.Programs[1].Post)
	}
	if got.Dir != td {
		t.Errorf("Dir = %q, want %q", got.Dir, td)
	}
}

func TestLoadMissingNameIsError(t *testing.T) {
	dir := t.TempDir()
	td := writeTheme(t, dir, "broken", `family = "x"`)
	if _, err := Load(td); err == nil {
		t.Fatal("expected error for theme without name")
	}
}

func TestLoadNoProgramsIsError(t *testing.T) {
	dir := t.TempDir()
	td := writeTheme(t, dir, "empty", `name = "Empty"`)
	if _, err := Load(td); err == nil {
		t.Fatal("expected error for theme with no programs")
	}
}

func TestDiscoverSortsBySlug(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "zzz", "name=\"Z\"\n[[programs]]\nname=\"vim\"\nfile=\"a\"\n")
	writeTheme(t, dir, "aaa", "name=\"A\"\n[[programs]]\nname=\"vim\"\nfile=\"a\"\n")
	// a directory without theme.toml must be ignored
	if err := os.MkdirAll(filepath.Join(dir, "notatheme"), 0o755); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("Discover on missing dir should be empty, got %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("got %d, want 0", len(themes))
	}
}

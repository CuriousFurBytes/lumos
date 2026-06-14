package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeGitURL(t *testing.T) {
	cases := map[string]string{
		"CuriousFurBytes/lumos-themes":   "https://github.com/CuriousFurBytes/lumos-themes",
		"github.com/foo/bar":             "https://github.com/foo/bar",
		"https://github.com/foo/bar":     "https://github.com/foo/bar",
		"https://github.com/foo/bar.git": "https://github.com/foo/bar.git",
		"git@github.com:foo/bar.git":     "git@github.com:foo/bar.git",
		"https://gitlab.com/foo/bar":     "https://gitlab.com/foo/bar",
	}
	for in, want := range cases {
		if got := NormalizeGitURL(in); got != want {
			t.Errorf("NormalizeGitURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func themeYAML(name string) string {
	return "name: \"" + name + "\"\nprograms:\n  - {name: vim, content: \"colors\"}\nvariants:\n  - {id: dark, name: Dark}\n"
}

// makeThemeFile writes a theme file in its own dir and returns the file path.
func makeThemeFile(t *testing.T, slug string) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, slug+".yaml")
	if err := os.WriteFile(p, []byte(themeYAML(slug)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeCloner copies a local fixture dir instead of cloning over the network.
type fakeCloner struct {
	fixture string
	cloned  int
}

func (f *fakeCloner) Clone(url, dest string) error {
	f.cloned++
	return copyTree(f.fixture, dest)
}

func TestInstallFromLocalFile(t *testing.T) {
	file := makeThemeFile(t, "dracula")
	themesDir := t.TempDir()

	slugs, err := Install(file, themesDir, &fakeCloner{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 1 || slugs[0] != "dracula" {
		t.Fatalf("slugs = %v", slugs)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "dracula.yaml")); err != nil {
		t.Errorf("theme not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "dracula"+originExt)); err != nil {
		t.Errorf("origin sidecar not written: %v", err)
	}
}

func TestInstallFromLocalFolder(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(themeYAML("a")), 0o644)
	os.MkdirAll(filepath.Join(dir, "themes"), 0o755)
	os.WriteFile(filepath.Join(dir, "themes", "b.yml"), []byte(themeYAML("b")), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a theme"), 0o644)

	themesDir := t.TempDir()
	slugs, err := Install(dir, themesDir, &fakeCloner{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 2 {
		t.Fatalf("slugs = %v, want 2", slugs)
	}
}

func TestInstallFromGitRepoUsesCloner(t *testing.T) {
	fixture := t.TempDir()
	os.WriteFile(filepath.Join(fixture, "rose-pine.yaml"), []byte(themeYAML("rose-pine")), 0o644)
	themesDir := t.TempDir()
	cloner := &fakeCloner{fixture: fixture}

	slugs, err := Install("CuriousFurBytes/rose-pine", themesDir, cloner)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if cloner.cloned != 1 {
		t.Errorf("expected 1 clone, got %d", cloner.cloned)
	}
	if len(slugs) != 1 || slugs[0] != "rose-pine" {
		t.Fatalf("slugs = %v", slugs)
	}
}

func TestInstallRejectsSourceWithoutTheme(t *testing.T) {
	empty := t.TempDir()
	if _, err := Install(empty, t.TempDir(), &fakeCloner{}); err == nil {
		t.Fatal("expected error installing a folder with no theme files")
	}
}

func TestUpdateLocalReCopies(t *testing.T) {
	// Install from a local file, then change the source and update.
	src := makeThemeFile(t, "catppuccin")
	themesDir := t.TempDir()
	if _, err := Install(src, themesDir, &fakeCloner{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(themeYAML("catppuccin")+"# edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update("catppuccin", themesDir, &fakeCloner{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(themesDir, "catppuccin.yaml"))
	if !contains(string(got), "# edited") {
		t.Errorf("update did not refresh content: %q", got)
	}
}

func TestUpdateGitReClones(t *testing.T) {
	fixture := t.TempDir()
	os.WriteFile(filepath.Join(fixture, "moon.yaml"), []byte(themeYAML("moon")), 0o644)
	themesDir := t.TempDir()
	cloner := &fakeCloner{fixture: fixture}
	if _, err := Install("foo/moon", themesDir, cloner); err != nil {
		t.Fatal(err)
	}
	if err := Update("moon", themesDir, cloner); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if cloner.cloned != 2 { // once for install, once for update
		t.Errorf("expected 2 clones, got %d", cloner.cloned)
	}
}

func TestUpdateUnknownTheme(t *testing.T) {
	if err := Update("ghost", t.TempDir(), &fakeCloner{}); err == nil {
		t.Fatal("expected error updating unknown theme")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}

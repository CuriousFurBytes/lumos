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

// makeThemeRepo creates a fake source repo containing one theme bundle.
func makeThemeRepo(t *testing.T, slug string) string {
	t.Helper()
	root := t.TempDir()
	td := filepath.Join(root, slug)
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name = \"" + slug + "\"\n[[programs]]\nname = \"vim\"\nfile = \"vim.vim\"\n"
	if err := os.WriteFile(filepath.Join(td, "theme.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "vim.vim"), []byte("\" colors"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// fakeCloner copies from a local fixture instead of hitting the network.
type fakeCloner struct {
	fixture string
	pulled  []string
}

func (f *fakeCloner) Clone(url, dest string) error { return copyTree(f.fixture, dest) }
func (f *fakeCloner) Pull(dir string) error        { f.pulled = append(f.pulled, dir); return nil }

func TestInstallFromLocalFolder(t *testing.T) {
	repo := makeThemeRepo(t, "dracula")
	themesDir := t.TempDir()

	slugs, err := Install(filepath.Join(repo, "dracula"), themesDir, &fakeCloner{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 1 || slugs[0] != "dracula" {
		t.Fatalf("slugs = %v", slugs)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "dracula", "theme.toml")); err != nil {
		t.Errorf("theme not installed: %v", err)
	}
	// origin must be recorded so update knows where to pull from.
	if _, err := os.Stat(filepath.Join(themesDir, "dracula", originFile)); err != nil {
		t.Errorf("origin file not written: %v", err)
	}
}

func TestInstallFromGitRepoUsesCloner(t *testing.T) {
	fixture := makeThemeRepo(t, "rose-pine")
	themesDir := t.TempDir()
	cloner := &fakeCloner{fixture: fixture}

	slugs, err := Install("CuriousFurBytes/rose-pine", themesDir, cloner)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 1 || slugs[0] != "rose-pine" {
		t.Fatalf("slugs = %v", slugs)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "rose-pine", "vim.vim")); err != nil {
		t.Errorf("theme file missing: %v", err)
	}
}

func TestInstallRejectsFolderWithoutTheme(t *testing.T) {
	empty := t.TempDir()
	if _, err := Install(empty, t.TempDir(), &fakeCloner{}); err == nil {
		t.Fatal("expected error installing a folder with no theme.toml")
	}
}

func TestUpdateSingleThemePulls(t *testing.T) {
	repo := makeThemeRepo(t, "catppuccin")
	themesDir := t.TempDir()
	if _, err := Install("foo/catppuccin", themesDir, &fakeCloner{fixture: repo}); err != nil {
		t.Fatal(err)
	}
	cloner := &fakeCloner{}
	if err := Update("catppuccin", themesDir, cloner); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(cloner.pulled) != 1 {
		t.Errorf("expected 1 pull, got %v", cloner.pulled)
	}
}

func TestUpdateUnknownTheme(t *testing.T) {
	if err := Update("ghost", t.TempDir(), &fakeCloner{}); err == nil {
		t.Fatal("expected error updating unknown theme")
	}
}

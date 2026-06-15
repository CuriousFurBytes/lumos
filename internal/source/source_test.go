package source

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
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

func manifest(name string) string {
	return "name: \"" + name + "\"\nvariants:\n  - {id: dark, name: Dark, colors: {}}\n"
}

// makeBundleDir writes an unpacked theme bundle directory.
func makeBundleDir(t *testing.T, parent, slug string) string {
	t.Helper()
	dir := filepath.Join(parent, slug)
	if err := os.MkdirAll(filepath.Join(dir, "programs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.yaml"), []byte(manifest(slug)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "programs", "vim.vim"), []byte("\" colors"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeBundleZip(t *testing.T, dir, dest string) string {
	t.Helper()
	zpath := filepath.Join(dest, filepath.Base(dir)+".zip")
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
		w, _ := zw.Create(filepath.ToSlash(rel))
		data, _ := os.ReadFile(p)
		_, err = w.Write(data)
		return err
	})
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zpath
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

func installedZipIsLoadable(t *testing.T, zpath string) {
	t.Helper()
	zr, err := zip.OpenReader(zpath)
	if err != nil {
		t.Fatalf("installed zip not readable: %v", err)
	}
	zr.Close()
}

func TestInstallFromLocalZip(t *testing.T) {
	tmp := t.TempDir()
	dir := makeBundleDir(t, tmp, "dracula")
	zpath := makeBundleZip(t, dir, tmp)
	themesDir := t.TempDir()

	slugs, err := Install(zpath, themesDir, &fakeCloner{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 1 || slugs[0] != "dracula" {
		t.Fatalf("slugs = %v", slugs)
	}
	out := filepath.Join(themesDir, "dracula.zip")
	installedZipIsLoadable(t, out)
	if _, err := os.Stat(filepath.Join(themesDir, "dracula"+originExt)); err != nil {
		t.Errorf("origin sidecar missing: %v", err)
	}
}

func TestInstallFromBundleDirectoryZipsIt(t *testing.T) {
	dir := makeBundleDir(t, t.TempDir(), "rose-pine")
	themesDir := t.TempDir()

	slugs, err := Install(dir, themesDir, &fakeCloner{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 1 || slugs[0] != "rose-pine" {
		t.Fatalf("slugs = %v", slugs)
	}
	installedZipIsLoadable(t, filepath.Join(themesDir, "rose-pine.zip"))
}

func TestInstallFromFolderOfBundles(t *testing.T) {
	src := t.TempDir()
	makeBundleDir(t, src, "a")
	makeBundleDir(t, src, "b")
	os.WriteFile(filepath.Join(src, "README.md"), []byte("noise"), 0o644)
	themesDir := t.TempDir()

	slugs, err := Install(src, themesDir, &fakeCloner{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(slugs) != 2 {
		t.Fatalf("slugs = %v, want 2", slugs)
	}
}

func TestInstallFromGitRepoUsesCloner(t *testing.T) {
	fixture := t.TempDir()
	makeBundleDir(t, fixture, "moon")
	themesDir := t.TempDir()
	cloner := &fakeCloner{fixture: fixture}

	slugs, err := Install("CuriousFurBytes/moon", themesDir, cloner)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if cloner.cloned != 1 {
		t.Errorf("expected 1 clone, got %d", cloner.cloned)
	}
	if len(slugs) != 1 || slugs[0] != "moon" {
		t.Fatalf("slugs = %v", slugs)
	}
}

func TestInstallRejectsSourceWithoutBundle(t *testing.T) {
	empty := t.TempDir()
	os.WriteFile(filepath.Join(empty, "readme.txt"), []byte("x"), 0o644)
	if _, err := Install(empty, t.TempDir(), &fakeCloner{}); err == nil {
		t.Fatal("expected error installing a folder with no bundles")
	}
}

func TestUpdateLocalReinstalls(t *testing.T) {
	dir := makeBundleDir(t, t.TempDir(), "catppuccin")
	themesDir := t.TempDir()
	if _, err := Install(dir, themesDir, &fakeCloner{}); err != nil {
		t.Fatal(err)
	}
	// Change the source manifest, then update.
	newManifest := manifest("catppuccin") + "description: edited\n"
	if err := os.WriteFile(filepath.Join(dir, "theme.yaml"), []byte(newManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update("catppuccin", themesDir, &fakeCloner{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !zipContains(t, filepath.Join(themesDir, "catppuccin.zip"), "theme.yaml", "edited") {
		t.Error("update did not refresh the bundle")
	}
}

func TestUpdateGitReClones(t *testing.T) {
	fixture := t.TempDir()
	makeBundleDir(t, fixture, "moon")
	themesDir := t.TempDir()
	cloner := &fakeCloner{fixture: fixture}
	if _, err := Install("foo/moon", themesDir, cloner); err != nil {
		t.Fatal(err)
	}
	if err := Update("moon", themesDir, cloner); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if cloner.cloned != 2 {
		t.Errorf("expected 2 clones, got %d", cloner.cloned)
	}
}

func TestUpdateUnknownTheme(t *testing.T) {
	if err := Update("ghost", t.TempDir(), &fakeCloner{}); err == nil {
		t.Fatal("expected error updating unknown theme")
	}
}

func zipContains(t *testing.T, zpath, entry, substr string) bool {
	t.Helper()
	zr, err := zip.OpenReader(zpath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != entry {
			continue
		}
		rc, _ := f.Open()
		buf := new(strings.Builder)
		io := make([]byte, 4096)
		for {
			n, err := rc.Read(io)
			buf.Write(io[:n])
			if err != nil {
				break
			}
		}
		rc.Close()
		return strings.Contains(buf.String(), substr)
	}
	return false
}

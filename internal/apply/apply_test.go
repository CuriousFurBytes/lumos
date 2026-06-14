package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

func fixtureTheme(t *testing.T) (theme.Theme, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "alacritty.toml"), []byte("# colors"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "bat.tmTheme"), []byte("<theme/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	th := theme.Theme{
		Name:   "Catppuccin Mocha",
		Slug:   "catppuccin-mocha",
		Family: "catppuccin",
		Dir:    bundle,
		Programs: []theme.Program{
			{Name: "alacritty", File: "alacritty.toml"}, // target from registry
			{Name: "bat", File: "bat.tmTheme"},          // registry default + post
		},
	}
	paths := config.Paths{
		Home:   dir,
		Config: filepath.Join(dir, "cfg"),
		Data:   filepath.Join(dir, "data"),
		State:  filepath.Join(dir, "state"),
		Cache:  filepath.Join(dir, "cache"),
	}
	return th, paths
}

func TestResolveFillsTargetFromRegistry(t *testing.T) {
	th, paths := fixtureTheme(t)
	progs, err := Resolve(th, paths)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(progs) != 2 {
		t.Fatalf("got %d programs", len(progs))
	}
	wantAlac := filepath.Join(paths.Config, "alacritty", "themes", "catppuccin-mocha.toml")
	if progs[0].Target != wantAlac {
		t.Errorf("alacritty target = %q, want %q", progs[0].Target, wantAlac)
	}
	// bat: ${name} placeholder + inherited post command from registry.
	wantBat := filepath.Join(paths.Config, "bat", "themes", "Catppuccin Mocha.tmTheme")
	if progs[1].Target != wantBat {
		t.Errorf("bat target = %q, want %q", progs[1].Target, wantBat)
	}
	if len(progs[1].Post) != 1 || progs[1].Post[0] != "bat cache --build" {
		t.Errorf("bat post = %v, want [bat cache --build]", progs[1].Post)
	}
}

func TestResolveExplicitTargetWins(t *testing.T) {
	th, paths := fixtureTheme(t)
	th.Programs[0].Target = "~/custom/alac.toml"
	progs, err := Resolve(th, paths)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(paths.Home, "custom", "alac.toml")
	if progs[0].Target != want {
		t.Errorf("target = %q, want %q", progs[0].Target, want)
	}
}

func TestResolveUnknownPortWithoutTargetErrors(t *testing.T) {
	th, paths := fixtureTheme(t)
	th.Programs = append(th.Programs, theme.Program{Name: "totally-unknown", File: "x"})
	if _, err := Resolve(th, paths); err == nil {
		t.Fatal("expected error: unknown port with no target")
	}
}

type fakeRunner struct{ ran []string }

func (f *fakeRunner) Run(cmd string) error { f.ran = append(f.ran, cmd); return nil }

func TestApplyWritesFilesAndRunsPost(t *testing.T) {
	th, paths := fixtureTheme(t)
	progs, err := Resolve(th, paths)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	rep, err := Apply(progs, runner, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(rep.Applied) != 2 {
		t.Errorf("applied = %v", rep.Applied)
	}
	data, err := os.ReadFile(progs[0].Target)
	if err != nil || string(data) != "# colors" {
		t.Errorf("alacritty file not copied: %v %q", err, data)
	}
	if len(runner.ran) != 1 || runner.ran[0] != "bat cache --build" {
		t.Errorf("post commands ran = %v", runner.ran)
	}
}

func TestApplyDryRunWritesNothing(t *testing.T) {
	th, paths := fixtureTheme(t)
	progs, err := Resolve(th, paths)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	if _, err := Apply(progs, runner, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(progs[0].Target); !os.IsNotExist(err) {
		t.Error("dry run should not write files")
	}
	if len(runner.ran) != 0 {
		t.Errorf("dry run should not run commands, ran %v", runner.ran)
	}
}

type failRunner struct{ ran []string }

func (f *failRunner) Run(cmd string) error {
	f.ran = append(f.ran, cmd)
	return os.ErrPermission
}

func TestApplyPostFailureIsNonFatal(t *testing.T) {
	th, paths := fixtureTheme(t)
	progs, err := Resolve(th, paths)
	if err != nil {
		t.Fatal(err)
	}
	runner := &failRunner{}
	rep, err := Apply(progs, runner, false)
	if err != nil {
		t.Fatalf("a failing reload hook must not fail the apply: %v", err)
	}
	// The theme file is still written even though the hook failed.
	if !fileExistsAt(progs[1].Target) {
		t.Error("bat theme file should still be written")
	}
	if len(rep.Warnings) == 0 {
		t.Error("expected a warning recorded for the failed hook")
	}
}

func fileExistsAt(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestApplyMissingSourceErrors(t *testing.T) {
	th, paths := fixtureTheme(t)
	th.Programs[0].File = "missing.toml"
	progs, err := Resolve(th, paths)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(progs, &fakeRunner{}, false); err == nil {
		t.Fatal("expected error for missing source file")
	}
}

package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

func fixture(t *testing.T) (theme.Theme, theme.Variant, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	th := theme.Theme{
		Name: "Catppuccin",
		Slug: "catppuccin",
		Programs: []theme.Program{
			{Port: "alacritty", Template: "bg = \"${color.base}\"\nfg = \"${color.text}\""},
			{Port: "bat", Template: "static bat theme"},
		},
		Variants: []theme.Variant{
			{ID: "mocha", Name: "Mocha", Style: "dark", Colors: map[string]string{
				"base": "#1e1e2e", "text": "#cdd6f4",
			}},
		},
	}
	paths := config.Paths{
		Home: dir, Config: filepath.Join(dir, "cfg"), Data: filepath.Join(dir, "data"),
		State: filepath.Join(dir, "state"), Cache: filepath.Join(dir, "cache"),
	}
	return th, th.Variants[0], paths
}

func TestRenderSubstitutesPaletteAndTarget(t *testing.T) {
	th, v, paths := fixture(t)
	progs, err := Render(th, v, paths)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(progs) != 2 {
		t.Fatalf("got %d", len(progs))
	}
	var alac ResolvedProgram
	for _, p := range progs {
		if p.Name == "alacritty" {
			alac = p
		}
	}
	wantTarget := filepath.Join(paths.Config, "alacritty", "themes", "catppuccin-mocha.toml")
	if alac.Target != wantTarget {
		t.Errorf("target = %q, want %q", alac.Target, wantTarget)
	}
	if alac.Content != "bg = \"#1e1e2e\"\nfg = \"#cdd6f4\"" {
		t.Errorf("rendered content = %q", alac.Content)
	}
}

func TestRenderInheritsRegistryPost(t *testing.T) {
	th, v, paths := fixture(t)
	progs, err := Render(th, v, paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progs {
		if p.Name == "bat" {
			if len(p.Post) != 1 || p.Post[0] != "bat cache --build" {
				t.Errorf("bat post = %v, want registry default", p.Post)
			}
		}
	}
}

func TestRenderMissingColorErrors(t *testing.T) {
	th, v, paths := fixture(t)
	th.Programs[0].Template = "x = ${color.nonexistent}"
	if _, err := Render(th, v, paths); err == nil {
		t.Fatal("expected error for undefined palette key")
	}
}

func TestRenderUnknownPortErrors(t *testing.T) {
	th, v, paths := fixture(t)
	th.Programs = append(th.Programs, theme.Program{Port: "mystery", Template: "x"})
	if _, err := Render(th, v, paths); err == nil {
		t.Fatal("expected error: unknown port (no registry target)")
	}
}

type fakeRunner struct{ ran []string }

func (f *fakeRunner) Run(cmd string) error { f.ran = append(f.ran, cmd); return nil }

func TestApplyWritesRenderedContent(t *testing.T) {
	th, v, paths := fixture(t)
	progs, _ := Render(th, v, paths)
	runner := &fakeRunner{}
	rep, err := Apply(progs, runner, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(rep.Applied) != 2 {
		t.Errorf("applied = %v", rep.Applied)
	}
	var alac ResolvedProgram
	for _, p := range progs {
		if p.Name == "alacritty" {
			alac = p
		}
	}
	got, err := os.ReadFile(alac.Target)
	if err != nil || string(got) != alac.Content {
		t.Errorf("file = %q err=%v", got, err)
	}
	if len(runner.ran) != 1 {
		t.Errorf("hooks ran = %v", runner.ran)
	}
}

func TestApplyDryRunWritesNothing(t *testing.T) {
	th, v, paths := fixture(t)
	progs, _ := Render(th, v, paths)
	runner := &fakeRunner{}
	if _, err := Apply(progs, runner, true); err != nil {
		t.Fatal(err)
	}
	for _, p := range progs {
		if _, err := os.Stat(p.Target); !os.IsNotExist(err) {
			t.Errorf("dry run wrote %s", p.Target)
		}
	}
	if len(runner.ran) != 0 {
		t.Error("dry run ran hooks")
	}
}

type failRunner struct{}

func (failRunner) Run(string) error { return os.ErrPermission }

func TestApplyPostFailureIsNonFatal(t *testing.T) {
	th, v, paths := fixture(t)
	progs, _ := Render(th, v, paths)
	rep, err := Apply(progs, failRunner{}, false)
	if err != nil {
		t.Fatalf("failing hook must not fail apply: %v", err)
	}
	if len(rep.Warnings) == 0 {
		t.Error("expected a warning for failed hook")
	}
}

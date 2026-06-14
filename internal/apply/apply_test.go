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
			{Name: "alacritty", Template: "bg = \"${color.base}\"\nfg = \"${color.text}\""},
			{Name: "bat", Content: "static", Post: []string{"bat cache --build"}},
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
	wantTarget := filepath.Join(paths.Config, "alacritty", "themes", "catppuccin-mocha.toml")
	if progs[0].Target != wantTarget {
		t.Errorf("target = %q, want %q", progs[0].Target, wantTarget)
	}
	if progs[0].Content != "bg = \"#1e1e2e\"\nfg = \"#cdd6f4\"" {
		t.Errorf("rendered content = %q", progs[0].Content)
	}
	// bat: literal content + inherited post hook from registry/theme.
	if progs[1].Content != "static" {
		t.Errorf("bat content = %q", progs[1].Content)
	}
	if len(progs[1].Post) != 1 {
		t.Errorf("bat post = %v", progs[1].Post)
	}
}

func TestRenderUsesVariantInTargetPlaceholder(t *testing.T) {
	th, v, paths := fixture(t)
	th.Programs[0].Target = "~/c/${slug}-${variant}.toml"
	progs, err := Render(th, v, paths)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(paths.Home, "c", "catppuccin-mocha.toml")
	if progs[0].Target != want {
		t.Errorf("target = %q, want %q", progs[0].Target, want)
	}
}

func TestRenderMissingColorErrors(t *testing.T) {
	th, v, paths := fixture(t)
	th.Programs[0].Template = "x = ${color.nonexistent}"
	if _, err := Render(th, v, paths); err == nil {
		t.Fatal("expected error for undefined palette key")
	}
}

func TestRenderUnknownPortWithoutTargetErrors(t *testing.T) {
	th, v, paths := fixture(t)
	th.Programs = append(th.Programs, theme.Program{Name: "mystery", Content: "x"})
	if _, err := Render(th, v, paths); err == nil {
		t.Fatal("expected error: unknown port with no target")
	}
}

type fakeRunner struct{ ran []string }

func (f *fakeRunner) Run(cmd string) error { f.ran = append(f.ran, cmd); return nil }

func TestApplyWritesRenderedContent(t *testing.T) {
	th, v, paths := fixture(t)
	progs, err := Render(th, v, paths)
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
	got, err := os.ReadFile(progs[0].Target)
	if err != nil || string(got) != progs[0].Content {
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
	if _, err := os.Stat(progs[0].Target); !os.IsNotExist(err) {
		t.Error("dry run wrote a file")
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

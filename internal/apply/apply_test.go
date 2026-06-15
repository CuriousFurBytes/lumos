package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/registry"
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

// installSet is a Detector that reports a fixed set of commands as installed.
type installSet map[string]bool

func (s installSet) Installed(command string) bool { return s[command] }

// allInstalled treats every program as present on the system, which keeps the
// rendering-focused tests independent of what the host machine has installed.
var allInstalled = installSet{"alacritty": true, "bat": true, "mystery": true}

func TestRenderSubstitutesPaletteAndTarget(t *testing.T) {
	th, v, paths := fixture(t)
	progs, _, err := Render(th, v, paths, nil, allInstalled)
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
	progs, _, err := Render(th, v, paths, nil, allInstalled)
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
	if _, _, err := Render(th, v, paths, nil, allInstalled); err == nil {
		t.Fatal("expected error for undefined palette key")
	}
}

func TestRenderUnknownPortErrors(t *testing.T) {
	th, v, paths := fixture(t)
	th.Programs = append(th.Programs, theme.Program{Port: "mystery", Template: "x"})
	if _, _, err := Render(th, v, paths, nil, allInstalled); err == nil {
		t.Fatal("expected error: unknown port (no registry target)")
	}
}

func TestRenderSkipsUninstalledPorts(t *testing.T) {
	// bat is not installed on this system, so only alacritty is rendered and
	// bat is reported as skipped rather than themed.
	th, v, paths := fixture(t)
	progs, skipped, err := Render(th, v, paths, nil, installSet{"alacritty": true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(progs) != 1 || progs[0].Name != "alacritty" {
		t.Fatalf("rendered = %v, want only alacritty", progs)
	}
	if len(skipped) != 1 || skipped[0] != "bat" {
		t.Errorf("skipped = %v, want [bat]", skipped)
	}
}

func TestRenderSkipsByDetectBinaryNotPortKey(t *testing.T) {
	// A program is installed when its detect binary is on PATH, which can
	// differ from the port key. The detector is queried with the command, so
	// having only the port key present must NOT count as installed.
	th, v, paths := fixture(t)
	th.Programs = []theme.Program{{Port: "wezterm-lua", Template: "x"}}
	progs, skipped, err := Render(th, v, paths, nil, installSet{"wezterm-lua": true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(progs) != 0 {
		t.Errorf("rendered = %v, want none (only port key present, not binary)", progs)
	}
	if len(skipped) != 1 || skipped[0] != "wezterm-lua" {
		t.Errorf("skipped = %v, want [wezterm-lua]", skipped)
	}

	progs, skipped, err = Render(th, v, paths, nil, installSet{"wezterm": true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(progs) != 1 || progs[0].Name != "wezterm-lua" {
		t.Errorf("rendered = %v, want wezterm-lua when binary present", progs)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
}

func TestRenderUnknownPortBeforeInstallCheck(t *testing.T) {
	// An unknown port is still a hard error regardless of installation: a
	// theme that references a port lumos cannot place is malformed.
	th, v, paths := fixture(t)
	th.Programs = []theme.Program{{Port: "mystery", Template: "x"}}
	if _, _, err := Render(th, v, paths, nil, installSet{}); err == nil {
		t.Fatal("expected error for unknown port even when not installed")
	}
}

func TestPathDetectorReportsLookPath(t *testing.T) {
	// The real detector uses PATH. `sh` is essentially always present in the
	// test environment; a clearly bogus name never is.
	var d PathDetector
	if !d.Installed("sh") {
		t.Error("expected sh to be detected on PATH")
	}
	if d.Installed("lumos-definitely-not-a-real-binary-xyz") {
		t.Error("bogus binary unexpectedly reported as installed")
	}
}

func TestRenderResolvesCustomPort(t *testing.T) {
	// A program whose port is not in the embedded base resolves when the
	// caller supplies a custom port set (the use case for issue #6: programs
	// users add themselves). Its target and install steps come from the
	// custom entry. allInstalled keeps the install check out of the way.
	th, v, paths := fixture(t)
	th.Programs = []theme.Program{{Port: "cava", Template: "bg = ${color.base}"}}
	ports := registry.Merge(map[string]registry.Port{
		"cava": {
			Name:   "cava",
			Target: "${XDG_CONFIG_HOME}/cava/themes/${slug}-${variant}.conf",
			Post:   []string{"pkill -USR2 cava"},
		},
	})

	progs, _, err := Render(th, v, paths, ports, installSet{"cava": true})
	if err != nil {
		t.Fatalf("Render with custom port: %v", err)
	}
	if len(progs) != 1 {
		t.Fatalf("got %d programs", len(progs))
	}
	want := filepath.Join(paths.Config, "cava", "themes", "catppuccin-mocha.conf")
	if progs[0].Target != want {
		t.Errorf("target = %q, want %q", progs[0].Target, want)
	}
	if progs[0].Content != "bg = #1e1e2e" {
		t.Errorf("content = %q", progs[0].Content)
	}
	if len(progs[0].Post) != 1 || progs[0].Post[0] != "pkill -USR2 cava" {
		t.Errorf("post = %v, want custom install step", progs[0].Post)
	}
}

func TestRenderCustomPortOverridesBuiltin(t *testing.T) {
	// A custom port keyed like a built-in overrides the built-in target,
	// letting users redirect where lumos installs a known program's theme.
	th, v, paths := fixture(t)
	th.Programs = []theme.Program{{Port: "alacritty", Template: "x"}}
	ports := registry.Merge(map[string]registry.Port{
		"alacritty": {Name: "Alacritty", Target: "${XDG_CONFIG_HOME}/custom/${slug}.toml"},
	})
	progs, _, err := Render(th, v, paths, ports, installSet{"alacritty": true})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(paths.Config, "custom", "catppuccin.toml")
	if progs[0].Target != want {
		t.Errorf("target = %q, want override %q", progs[0].Target, want)
	}
}

type fakeRunner struct{ ran []string }

func (f *fakeRunner) Run(cmd string) error { f.ran = append(f.ran, cmd); return nil }

func TestApplyWritesRenderedContent(t *testing.T) {
	th, v, paths := fixture(t)
	progs, _, _ := Render(th, v, paths, nil, allInstalled)
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
	progs, _, _ := Render(th, v, paths, nil, allInstalled)
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
	progs, _, _ := Render(th, v, paths, nil, allInstalled)
	rep, err := Apply(progs, failRunner{}, false)
	if err != nil {
		t.Fatalf("failing hook must not fail apply: %v", err)
	}
	if len(rep.Warnings) == 0 {
		t.Error("expected a warning for failed hook")
	}
}

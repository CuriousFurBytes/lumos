package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllLoadsEmbeddedPorts(t *testing.T) {
	all := All()
	if len(all) < 20 {
		t.Fatalf("expected a base of at least 20 ports, got %d", len(all))
	}
	// A few well-known ports we seeded from the upstream theme projects.
	for _, key := range []string{"alacritty", "kitty", "bat", "btop", "tmux", "helix"} {
		if _, ok := all[key]; !ok {
			t.Errorf("missing expected port %q", key)
		}
	}
}

func TestLookup(t *testing.T) {
	p, ok := Lookup("alacritty")
	if !ok {
		t.Fatal("alacritty not found")
	}
	if p.Target == "" {
		t.Error("alacritty port has no default target")
	}
	if len(p.Families) == 0 {
		t.Error("alacritty port lists no families")
	}
	if _, ok := Lookup("does-not-exist"); ok {
		t.Error("unexpected hit for unknown port")
	}
}

func TestEveryPortHasTarget(t *testing.T) {
	for key, p := range All() {
		if p.Target == "" {
			t.Errorf("port %q has empty target", key)
		}
		if p.Name == "" {
			t.Errorf("port %q has empty name", key)
		}
	}
}

// TestExpandedPortsPresent covers issue #4: a batch of additional programs
// requested for the port base. Each must be registered, looked up by its key
// and carry a non-empty display name, target and at least one family.
func TestExpandedPortsPresent(t *testing.T) {
	// Keys for every program named in issue #4. Programs already present in
	// the base before the expansion are included so the catalogue stays
	// complete and regressions are caught.
	wantKeys := []string{
		"asciinema",
		"aseprite",
		"atuin",
		"bottom",
		"chrome",
		"cider",
		"cosmic",
		"delta",
		"lazygit",
		"emacs",
		"vim",
		"nvim",
		"helix",
		"eza",
		"firefox",
		"fish",
		"jetbrains",
		"fluent-terminal",
		"fzf",
		"gh-dash",
		"ghostty",
		"gnome-terminal",
		"gnome-text-editor",
		"glamour",
		"hyprland",
		"insomnia",
		"iterm",
		"k9s",
		"konsole",
		"nix",
		"notepad-plus-plus",
		"obsidian",
		"ollama",
		"opencode",
		"posting",
		"sublime-text",
		"tabby",
		"tilix",
		"tmux",
		"visual-studio",
		"vscode",
		"warp",
		"waybar",
		"yazi",
		"zed",
		"zellij",
		"zen-browser",
		"zsh-syntax-highlighting",
		"zsh-fast-syntax-highlighting",
	}

	for _, key := range wantKeys {
		p, ok := Lookup(key)
		if !ok {
			t.Errorf("missing expected port %q", key)
			continue
		}
		if p.Name == "" {
			t.Errorf("port %q has empty name", key)
		}
		if p.Target == "" {
			t.Errorf("port %q has empty target", key)
		}
		if len(p.Families) == 0 {
			t.Errorf("port %q lists no families", key)
		}
	}
}

// TestEveryPortHasFamiliesAndCategories enforces the invariants every port in
// the base — original or newly added under issue #4 — must satisfy.
func TestEveryPortHasFamiliesAndCategories(t *testing.T) {
	valid := map[string]bool{
		"terminal": true, "cli": true, "editor": true, "browser": true,
		"desktop": true, "application": true,
	}
	for key, p := range All() {
		if len(p.Families) == 0 {
			t.Errorf("port %q lists no families", key)
		}
		if len(p.Categories) == 0 {
			t.Errorf("port %q lists no categories", key)
		}
		for _, c := range p.Categories {
			if !valid[c] {
				t.Errorf("port %q has unknown category %q", key, c)
			}
		}
	}
}

func TestCommandDefaultsToPortKey(t *testing.T) {
	// Most programs are detected by an executable that shares the port key
	// (alacritty, kitty, bat …), so Command falls back to that key.
	p := Port{}
	if got := p.Command("alacritty"); got != "alacritty" {
		t.Errorf("Command fallback = %q, want port key", got)
	}
}

func TestCommandUsesExplicitDetect(t *testing.T) {
	// Some ports install a file for a program whose binary differs from the
	// port key (e.g. wezterm-lua is provided by the `wezterm` binary).
	p := Port{Detect: "wezterm"}
	if got := p.Command("wezterm-lua"); got != "wezterm" {
		t.Errorf("Command = %q, want explicit detect binary", got)
	}
}

func TestPortsWithDistinctBinariesDeclareDetect(t *testing.T) {
	// Ports whose port key is not their executable name must declare an
	// explicit detect binary, otherwise lumos would never find them on PATH.
	want := map[string]string{
		"wezterm-lua": "wezterm",
		"nvim":        "nvim",
		"gtk":         "gsettings",
	}
	all := All()
	for key, bin := range want {
		p, ok := all[key]
		if !ok {
			t.Fatalf("missing port %q", key)
		}
		if got := p.Command(key); got != bin {
			t.Errorf("port %q command = %q, want %q", key, got, bin)
		}
	}
}

func TestLoadFileParsesCustomPorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports.toml")
	const data = `
[ports.cava]
name = "cava"
categories = ["cli"]
target = "${XDG_CONFIG_HOME}/cava/themes/${slug}-${variant}.conf"
post = ["pkill -USR2 cava"]
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	ports, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	cava, ok := ports["cava"]
	if !ok {
		t.Fatal("custom port cava not loaded")
	}
	if cava.Name != "cava" {
		t.Errorf("name = %q", cava.Name)
	}
	if cava.Target == "" {
		t.Error("custom port has no target")
	}
	if len(cava.Post) != 1 || cava.Post[0] != "pkill -USR2 cava" {
		t.Errorf("post = %v, want the install step", cava.Post)
	}
}

func TestLoadFileMissingReturnsEmpty(t *testing.T) {
	ports, err := LoadFile(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected no ports, got %d", len(ports))
	}
}

func TestLoadFileInvalidErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports.toml")
	if err := os.WriteFile(path, []byte("this is ][ not valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected a parse error for invalid ports.toml")
	}
}

func TestMergeOverlaysCustomPorts(t *testing.T) {
	custom := map[string]Port{
		// A brand-new port not in the embedded base.
		"cava": {Name: "cava", Target: "x"},
		// An override of an existing built-in port.
		"alacritty": {Name: "My Alacritty", Target: "custom/path"},
	}
	merged := Merge(custom)

	if _, ok := merged["cava"]; !ok {
		t.Error("custom port cava missing from merge")
	}
	if got := merged["alacritty"]; got.Name != "My Alacritty" || got.Target != "custom/path" {
		t.Errorf("custom override not applied: %+v", got)
	}
	// Untouched embedded ports survive.
	if _, ok := merged["kitty"]; !ok {
		t.Error("embedded port kitty lost in merge")
	}
	// Merge must not mutate the embedded base.
	if base, _ := Lookup("alacritty"); base.Name == "My Alacritty" {
		t.Error("Merge mutated the embedded registry")
	}
}

func TestMergeNilCustomReturnsBaseCopy(t *testing.T) {
	merged := Merge(nil)
	if len(merged) != len(All()) {
		t.Errorf("Merge(nil) size = %d, want embedded base size %d", len(merged), len(All()))
	}
}

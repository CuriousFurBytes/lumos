package registry

import "testing"

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

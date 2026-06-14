package config

import (
	"path/filepath"
	"testing"
)

func TestResolveRespectsXDGEnv(t *testing.T) {
	t.Setenv("HOME", "/home/tux")
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_DATA_HOME", "/data")
	t.Setenv("XDG_STATE_HOME", "/state")
	t.Setenv("XDG_CACHE_HOME", "/cache")

	p := Resolve()

	if p.Config != "/cfg" {
		t.Errorf("Config = %q, want /cfg", p.Config)
	}
	if got, want := p.ThemesDir(), "/cfg/lumos/themes"; got != want {
		t.Errorf("ThemesDir = %q, want %q", got, want)
	}
	if got, want := p.StateFile(), "/state/lumos/state.toml"; got != want {
		t.Errorf("StateFile = %q, want %q", got, want)
	}
}

func TestResolveFallsBackToHome(t *testing.T) {
	t.Setenv("HOME", "/home/tux")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	p := Resolve()

	if got, want := p.Config, "/home/tux/.config"; got != want {
		t.Errorf("Config = %q, want %q", got, want)
	}
	if got, want := p.State, "/home/tux/.local/state"; got != want {
		t.Errorf("State = %q, want %q", got, want)
	}
}

func TestExpand(t *testing.T) {
	p := Paths{
		Home:   "/home/tux",
		Config: "/home/tux/.config",
		Data:   "/home/tux/.local/share",
	}
	cases := map[string]string{
		"${XDG_CONFIG_HOME}/alacritty/x.toml": "/home/tux/.config/alacritty/x.toml",
		"~/.config/kitty":                     "/home/tux/.config/kitty",
		"$HOME/foo":                           "/home/tux/foo",
		"${XDG_DATA_HOME}/lumos":              "/home/tux/.local/share/lumos",
	}
	for in, want := range cases {
		if got := p.Expand(in); got != want {
			t.Errorf("Expand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "lumos", "state.toml")

	if err := SaveState(file, State{Current: "catppuccin-mocha"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState(file)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.Current != "catppuccin-mocha" {
		t.Errorf("Current = %q, want catppuccin-mocha", got.Current)
	}
}

func TestLoadStateMissingReturnsEmpty(t *testing.T) {
	got, err := LoadState(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("LoadState on missing file should not error, got %v", err)
	}
	if got.Current != "" {
		t.Errorf("Current = %q, want empty", got.Current)
	}
}

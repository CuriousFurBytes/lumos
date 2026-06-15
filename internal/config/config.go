// Package config resolves lumos' XDG base directories and persists the
// small amount of state lumos needs (which theme is currently selected).
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Paths holds the resolved base directories lumos works within.
type Paths struct {
	Home   string
	Config string
	Data   string
	State  string
	Cache  string
}

// Resolve reads the XDG_* environment variables, falling back to the
// conventional ~/.config, ~/.local/share, ~/.local/state and ~/.cache
// locations when they are unset.
func Resolve() Paths {
	home, _ := os.UserHomeDir()
	if env := os.Getenv("HOME"); env != "" {
		home = env
	}
	pick := func(env, fallback string) string {
		if v := os.Getenv(env); v != "" {
			return v
		}
		return filepath.Join(home, fallback)
	}
	return Paths{
		Home:   home,
		Config: pick("XDG_CONFIG_HOME", ".config"),
		Data:   pick("XDG_DATA_HOME", filepath.Join(".local", "share")),
		State:  pick("XDG_STATE_HOME", filepath.Join(".local", "state")),
		Cache:  pick("XDG_CACHE_HOME", ".cache"),
	}
}

// ThemesDir is where user and installed themes live.
func (p Paths) ThemesDir() string { return filepath.Join(p.Config, "lumos", "themes") }

// StateFile records the currently selected theme.
func (p Paths) StateFile() string { return filepath.Join(p.State, "lumos", "state.toml") }

// PortsFile is the optional user-defined port registry. Entries here add
// install targets (and reload/install shell steps via `post`) for programs
// lumos does not ship in its embedded base, and may override built-in ports.
func (p Paths) PortsFile() string { return filepath.Join(p.Config, "lumos", "ports.toml") }

// Expand resolves the placeholders lumos supports inside theme target
// paths: ~, $HOME, ${XDG_CONFIG_HOME}, ${XDG_DATA_HOME}, ${XDG_STATE_HOME}
// and ${XDG_CACHE_HOME}.
func (p Paths) Expand(s string) string {
	replacements := []struct{ from, to string }{
		{"${XDG_CONFIG_HOME}", p.Config},
		{"${XDG_DATA_HOME}", p.Data},
		{"${XDG_STATE_HOME}", p.State},
		{"${XDG_CACHE_HOME}", p.Cache},
		{"$HOME", p.Home},
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.from, r.to)
	}
	if s == "~" {
		return p.Home
	}
	if strings.HasPrefix(s, "~/") {
		s = filepath.Join(p.Home, s[2:])
	}
	return s
}

// State is the persisted selection.
type State struct {
	Current string `toml:"current"`
	Variant string `toml:"variant"`
}

// LoadState reads state from file. A missing file yields a zero State and
// no error so first runs behave gracefully.
func LoadState(file string) (State, error) {
	var s State
	_, err := toml.DecodeFile(file, &s)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	return s, err
}

// SaveState writes state to file, creating parent directories as needed.
func SaveState(file string, s State) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(s)
}

// Package registry holds lumos' embedded base of "ports": the catalogue of
// programs lumos knows how to theme and where each one expects its theme
// file to live.
//
// The base is seeded from the canonical port lists published by the
// Catppuccin, Dracula, Base16, Base24 and Rosé Pine projects (see
// ports.toml). It lets theme authors omit the target path for well-known
// programs — lumos fills in the conventional location automatically.
package registry

import (
	_ "embed"
	"sync"

	"github.com/BurntSushi/toml"
)

//go:embed ports.toml
var portsTOML []byte

// Port is the registry entry for a single program.
type Port struct {
	// Name is the human-friendly display name.
	Name string `toml:"name"`
	// Target is the default destination for the theme file. It supports
	// the config placeholders plus ${slug} and ${name}, resolved per theme
	// at apply time.
	Target string `toml:"target"`
	// Detect is the executable lumos looks for on $PATH to decide whether
	// the program is installed. It defaults to the port key, so it only
	// needs setting when the binary name differs (e.g. the wezterm-lua port
	// is provided by the `wezterm` binary).
	Detect string `toml:"detect"`
	// Post are default reload/rebuild commands for this program.
	Post []string `toml:"post"`
	// Categories mirror the upstream port categorisation (terminal, cli…).
	Categories []string `toml:"categories"`
	// Families lists the theme projects that publish a port for this
	// program, e.g. catppuccin, dracula, base16, base24, rose-pine.
	Families []string `toml:"families"`
}

// Command returns the executable lumos should look for on $PATH to decide
// whether this port's program is installed. It is the explicit Detect value
// when set, otherwise the port key (which for most programs is the binary
// name).
func (p Port) Command(portKey string) string {
	if p.Detect != "" {
		return p.Detect
	}
	return portKey
}

type file struct {
	Ports map[string]Port `toml:"ports"`
}

var (
	once   sync.Once
	parsed map[string]Port
)

func load() map[string]Port {
	once.Do(func() {
		var f file
		if err := toml.Unmarshal(portsTOML, &f); err != nil {
			// The data is embedded and covered by tests, so a parse error
			// here is a programming error, not a runtime condition.
			panic("registry: invalid embedded ports.toml: " + err.Error())
		}
		parsed = f.Ports
	})
	return parsed
}

// All returns the full port base keyed by port name.
func All() map[string]Port { return load() }

// Lookup returns the port registered under name, if any.
func Lookup(name string) (Port, bool) {
	p, ok := load()[name]
	return p, ok
}

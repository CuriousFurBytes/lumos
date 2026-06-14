// Package cli implements lumos' command-line interface: argument parsing,
// the interactive theme picker, and the install/update/enable flows.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/CuriousFurBytes/lumos/internal/apply"
	"github.com/CuriousFurBytes/lumos/internal/builtin"
	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/source"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

// Mode is the high-level action selected by the arguments.
type Mode int

// The available modes.
const (
	ModeInteractive Mode = iota
	ModeList
	ModeInstall
	ModeUpdate
	ModeEnable
	ModeHelp
	ModeVersion
)

// Options is the parsed command line.
type Options struct {
	Mode   Mode
	Spec   string // --install argument
	Name   string // theme name for enable/update
	Enable bool   // --enable
	DryRun bool   // --dry-run
}

// ParseArgs turns raw arguments into Options.
func ParseArgs(args []string) (Options, error) {
	opts := Options{Mode: ModeInteractive}
	var positional []string

	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-h", "--help":
			opts.Mode = ModeHelp
		case "--version":
			opts.Mode = ModeVersion
		case "--list", "-l":
			opts.Mode = ModeList
		case "--dry-run", "-n":
			opts.DryRun = true
		case "--enable":
			opts.Enable = true
		case "--install":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--install requires a github repo or local folder")
			}
			opts.Mode = ModeInstall
			i++
			opts.Spec = args[i]
		case "--update":
			opts.Mode = ModeUpdate
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				opts.Name = args[i]
			}
		default:
			if strings.HasPrefix(a, "-") {
				return opts, fmt.Errorf("unknown flag %q", a)
			}
			positional = append(positional, a)
		}
	}

	// Reconcile positionals with flags.
	switch {
	case opts.Mode == ModeInstall:
		// spec already captured; ignore extra positionals
	case len(positional) > 0:
		if opts.Mode == ModeInteractive {
			opts.Mode = ModeEnable
		}
		opts.Name = positional[0]
		// `--enable <name>` is just "enable this theme"; the flag is
		// only meaningful alongside --install.
		opts.Enable = false
	case opts.Enable && opts.Mode == ModeInteractive:
		return opts, fmt.Errorf("--enable requires a theme name or --install")
	}
	return opts, nil
}

// App carries the runtime dependencies, injectable for testing.
type App struct {
	Paths  config.Paths
	Runner apply.Runner
	Cloner source.Cloner
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
}

// New builds an App wired to the real environment.
func New() *App {
	return &App{
		Paths:  config.Resolve(),
		Runner: apply.ExecRunner{},
		Cloner: source.GitCloner{},
		In:     os.Stdin,
		Out:    os.Stdout,
		Err:    os.Stderr,
	}
}

// Run parses args and dispatches. It returns a process exit code.
func (a *App) Run(args []string) int {
	opts, err := ParseArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, "lumos:", err)
		fmt.Fprintln(a.Err, "try 'lumos --help'")
		return 2
	}

	switch opts.Mode {
	case ModeHelp:
		fmt.Fprint(a.Out, helpText)
		return 0
	case ModeVersion:
		fmt.Fprintln(a.Out, "lumos", Version)
		return 0
	}

	if _, err := builtin.Seed(a.Paths.ThemesDir()); err != nil {
		fmt.Fprintln(a.Err, "lumos: seeding starter themes:", err)
		return 1
	}

	var runErr error
	switch opts.Mode {
	case ModeList:
		runErr = a.list()
	case ModeInstall:
		runErr = a.install(opts)
	case ModeUpdate:
		runErr = a.update(opts)
	case ModeEnable:
		runErr = a.enable(opts.Name, opts.DryRun)
	case ModeInteractive:
		runErr = a.interactive(opts.DryRun)
	}
	if runErr != nil {
		fmt.Fprintln(a.Err, "lumos:", runErr)
		return 1
	}
	return 0
}

func (a *App) currentTheme() string {
	st, _ := config.LoadState(a.Paths.StateFile())
	return st.Current
}

func (a *App) list() error {
	themes, err := theme.Discover(a.Paths.ThemesDir())
	if err != nil {
		return err
	}
	if len(themes) == 0 {
		fmt.Fprintln(a.Out, "No themes found. Install one with 'lumos --install <repo|folder>'.")
		return nil
	}
	fmt.Fprint(a.Out, RenderThemeList(themes, a.currentTheme()))
	return nil
}

func (a *App) interactive(dryRun bool) error {
	themes, err := theme.Discover(a.Paths.ThemesDir())
	if err != nil {
		return err
	}
	if len(themes) == 0 {
		fmt.Fprintln(a.Out, "No themes found. Install one with 'lumos --install <repo|folder>'.")
		return nil
	}
	current := a.currentTheme()
	fmt.Fprint(a.Out, RenderThemeList(themes, current))
	fmt.Fprint(a.Out, "\nSelect a theme number (enter to cancel): ")

	line, _ := bufio.NewReader(a.In).ReadString('\n')
	idx, err := ParseSelection(strings.TrimSpace(line), len(themes))
	if err != nil {
		fmt.Fprintln(a.Out, "Cancelled.")
		return nil
	}
	return a.applyTheme(themes[idx], dryRun)
}

func (a *App) enable(name string, dryRun bool) error {
	themes, err := theme.Discover(a.Paths.ThemesDir())
	if err != nil {
		return err
	}
	for _, th := range themes {
		if th.Slug == name {
			return a.applyTheme(th, dryRun)
		}
	}
	return fmt.Errorf("theme %q not found (try 'lumos --list')", name)
}

func (a *App) applyTheme(th theme.Theme, dryRun bool) error {
	progs, err := apply.Resolve(th, a.Paths)
	if err != nil {
		return err
	}
	rep, err := apply.Apply(progs, a.Runner, dryRun)
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Fprintf(a.Out, "[dry-run] would apply %q to %d program(s): %s\n",
			th.Name, len(rep.Applied), strings.Join(rep.Applied, ", "))
		return nil
	}
	if err := config.SaveState(a.Paths.StateFile(), config.State{Current: th.Slug}); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Applied %q to %d program(s): %s\n",
		th.Name, len(rep.Applied), strings.Join(rep.Applied, ", "))
	for _, w := range rep.Warnings {
		fmt.Fprintln(a.Err, "  warning:", w)
	}
	return nil
}

func (a *App) install(opts Options) error {
	slugs, err := source.Install(opts.Spec, a.Paths.ThemesDir(), a.Cloner)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Installed: %s\n", strings.Join(slugs, ", "))
	if opts.Enable && len(slugs) > 0 {
		return a.enable(slugs[0], opts.DryRun)
	}
	return nil
}

func (a *App) update(opts Options) error {
	if err := source.Update(opts.Name, a.Paths.ThemesDir(), a.Cloner); err != nil {
		return err
	}
	if opts.Name == "" {
		fmt.Fprintln(a.Out, "Updated all themes.")
	} else {
		fmt.Fprintf(a.Out, "Updated %s.\n", opts.Name)
	}
	return nil
}

// RenderThemeList formats the picker list, marking the current theme.
func RenderThemeList(themes []theme.Theme, current string) string {
	var b strings.Builder
	b.WriteString("Available themes:\n")
	for i, th := range themes {
		marker := " "
		if th.Slug == current {
			marker = "*"
		}
		fmt.Fprintf(&b, " %s %d) %-22s %s (%d program(s))\n",
			marker, i+1, th.Slug, th.Name, len(th.Programs))
	}
	if current != "" {
		fmt.Fprintf(&b, "\nCurrent: %s\n", current)
	} else {
		b.WriteString("\nCurrent: (none)\n")
	}
	return b.String()
}

// ParseSelection validates a 1-based menu choice against n options and
// returns a 0-based index. An empty string means "cancel" and is an error.
func ParseSelection(input string, n int) (int, error) {
	if input == "" {
		return 0, fmt.Errorf("cancelled")
	}
	v, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("not a number: %q", input)
	}
	if v < 1 || v > n {
		return 0, fmt.Errorf("choice %d out of range 1..%d", v, n)
	}
	return v - 1, nil
}

const helpText = `lumos — switch themes across all your programs at once

USAGE:
    lumos                         Interactively pick a theme to apply
    lumos <name>                  Apply a theme by slug
    lumos --list                  List available themes and the current one
    lumos --install <src>         Install a theme from a github repo or local folder
    lumos --install <src> --enable    Install and immediately apply it
    lumos --update [name]         Update one theme, or all themes when omitted

FLAGS:
    --enable        With --install, apply the theme after installing
    --dry-run, -n   Show what would change without writing files
    --list,    -l   Non-interactive theme listing
    --version       Print version
    --help,    -h   Show this help

Themes live in $XDG_CONFIG_HOME/lumos/themes (default ~/.config/lumos/themes).
Drop your own theme bundles there to have lumos manage them.
`

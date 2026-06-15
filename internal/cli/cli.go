// Package cli implements lumos' command-line interface: argument parsing,
// the interactive theme/variant picker, and the install/update/enable flows.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/CuriousFurBytes/lumos/internal/apply"
	"github.com/CuriousFurBytes/lumos/internal/builtin"
	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/registry"
	"github.com/CuriousFurBytes/lumos/internal/source"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

// Version is overridden at build time via -ldflags. When unset (e.g. a plain
// `go install`), Version() falls back to the module version from the build
// info, so `go install ...@v0.0.1-alpha.1` still reports the right version.
var Version = "dev"

func resolveVersion() string {
	if Version != "dev" && Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return Version
}

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
	Mode    Mode
	Spec    string // --install argument
	Name    string // theme slug for enable/update
	Variant string // variant id for enable
	Enable  bool   // --enable
	DryRun  bool   // --dry-run
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
				return opts, fmt.Errorf("--install requires a github repo, folder or file")
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

	switch {
	case opts.Mode == ModeInstall:
		// spec already captured
	case len(positional) > 0:
		if opts.Mode == ModeInteractive {
			opts.Mode = ModeEnable
		}
		name, variant := splitThemeVariant(positional[0])
		opts.Name = name
		opts.Variant = variant
		if len(positional) > 1 && opts.Variant == "" {
			opts.Variant = positional[1]
		}
		opts.Enable = false // --enable is only meaningful with --install
	case opts.Enable && opts.Mode == ModeInteractive:
		return opts, fmt.Errorf("--enable requires a theme name or --install")
	}
	return opts, nil
}

// splitThemeVariant splits "theme/variant" into its parts.
func splitThemeVariant(s string) (theme, variant string) {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// App carries the runtime dependencies, injectable for testing.
type App struct {
	Paths    config.Paths
	Runner   apply.Runner
	Detector apply.Detector
	Cloner   source.Cloner
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
}

// New builds an App wired to the real environment.
func New() *App {
	return &App{
		Paths:    config.Resolve(),
		Runner:   apply.ExecRunner{},
		Detector: apply.PathDetector{},
		Cloner:   source.GitCloner{},
		In:       os.Stdin,
		Out:      os.Stdout,
		Err:      os.Stderr,
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
		fmt.Fprintln(a.Out, "lumos", resolveVersion())
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
		runErr = a.enable(opts.Name, opts.Variant, opts.DryRun)
	case ModeInteractive:
		runErr = a.interactive(opts.DryRun)
	}
	if runErr != nil {
		fmt.Fprintln(a.Err, "lumos:", runErr)
		return 1
	}
	return 0
}

func (a *App) state() config.State {
	st, _ := config.LoadState(a.Paths.StateFile())
	return st
}

func (a *App) list() error {
	themes, err := theme.Discover(a.Paths.ThemesDir())
	if err != nil {
		return err
	}
	if len(themes) == 0 {
		fmt.Fprintln(a.Out, "No themes found. Install one with 'lumos --install <repo|folder|zip>'.")
		return nil
	}
	fmt.Fprint(a.Out, RenderThemeList(themes, a.state().Current))
	return nil
}

func (a *App) interactive(dryRun bool) error {
	themes, err := theme.Discover(a.Paths.ThemesDir())
	if err != nil {
		return err
	}
	if len(themes) == 0 {
		fmt.Fprintln(a.Out, "No themes found. Install one with 'lumos --install <repo|folder|zip>'.")
		return nil
	}
	st := a.state()
	fmt.Fprint(a.Out, RenderThemeList(themes, st.Current))
	fmt.Fprint(a.Out, "\nSelect a theme number (enter to cancel): ")

	reader := bufio.NewReader(a.In)
	idx, err := readSelection(reader, len(themes))
	if err != nil {
		fmt.Fprintln(a.Out, "Cancelled.")
		return nil
	}
	th := themes[idx]

	v, err := a.chooseVariant(reader, th, st.Variant)
	if err != nil {
		fmt.Fprintln(a.Out, "Cancelled.")
		return nil
	}
	return a.applyVariant(th, v, dryRun)
}

// chooseVariant returns the variant to apply: the only one if a theme has a
// single variant, otherwise the user's interactive pick.
func (a *App) chooseVariant(reader *bufio.Reader, th theme.Theme, currentVariant string) (theme.Variant, error) {
	if len(th.Variants) == 1 {
		return th.Variants[0], nil
	}
	fmt.Fprint(a.Out, "\n"+RenderVariantList(th, currentVariant))
	fmt.Fprintf(a.Out, "\nSelect a variant for %s (enter to cancel): ", th.Name)
	idx, err := readSelection(reader, len(th.Variants))
	if err != nil {
		return theme.Variant{}, err
	}
	return th.Variants[idx], nil
}

func (a *App) enable(name, variant string, dryRun bool) error {
	themes, err := theme.Discover(a.Paths.ThemesDir())
	if err != nil {
		return err
	}
	for _, th := range themes {
		if th.Slug != name {
			continue
		}
		v, err := a.resolveVariant(th, variant)
		if err != nil {
			return err
		}
		return a.applyVariant(th, v, dryRun)
	}
	return fmt.Errorf("theme %q not found (try 'lumos --list')", name)
}

// resolveVariant picks the variant for a non-interactive enable: the named
// one if given, the only one if unambiguous, otherwise an interactive prompt.
func (a *App) resolveVariant(th theme.Theme, variant string) (theme.Variant, error) {
	if variant != "" {
		v, ok := th.Variant(variant)
		if !ok {
			return theme.Variant{}, fmt.Errorf("theme %q has no variant %q", th.Slug, variant)
		}
		return v, nil
	}
	if len(th.Variants) == 1 {
		return th.Variants[0], nil
	}
	return a.chooseVariant(bufio.NewReader(a.In), th, a.state().Variant)
}

// ports returns the embedded base merged with the user's custom ports from
// PortsFile, so themes can target programs lumos does not ship out of the box.
func (a *App) ports() (map[string]registry.Port, error) {
	custom, err := registry.LoadFile(a.Paths.PortsFile())
	if err != nil {
		return nil, err
	}
	return registry.Merge(custom), nil
}

func (a *App) applyVariant(th theme.Theme, v theme.Variant, dryRun bool) error {
	ports, err := a.ports()
	if err != nil {
		return err
	}
	progs, skipped, err := apply.Render(th, v, a.Paths, ports, a.Detector)
	if err != nil {
		return err
	}
	rep, err := apply.Apply(progs, a.Runner, dryRun)
	if err != nil {
		return err
	}
	label := fmt.Sprintf("%s (%s)", th.Name, v.Name)
	if dryRun {
		fmt.Fprintf(a.Out, "[dry-run] would apply %s to %d program(s): %s\n",
			label, len(rep.Applied), strings.Join(rep.Applied, ", "))
		a.reportSkipped(skipped)
		return nil
	}
	if err := config.SaveState(a.Paths.StateFile(), config.State{Current: th.Slug, Variant: v.ID}); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Applied %s to %d program(s): %s\n",
		label, len(rep.Applied), strings.Join(rep.Applied, ", "))
	a.reportSkipped(skipped)
	for _, w := range rep.Warnings {
		fmt.Fprintln(a.Err, "  warning:", w)
	}
	return nil
}

// reportSkipped tells the user which themed programs were left untouched
// because they are not installed on the system.
func (a *App) reportSkipped(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	fmt.Fprintf(a.Out, "Skipped %d not-installed program(s): %s\n",
		len(skipped), strings.Join(skipped, ", "))
}

func (a *App) install(opts Options) error {
	slugs, err := source.Install(opts.Spec, a.Paths.ThemesDir(), a.Cloner)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Installed: %s\n", strings.Join(slugs, ", "))
	if opts.Enable && len(slugs) > 0 {
		return a.enable(slugs[0], opts.Variant, opts.DryRun)
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
		fmt.Fprintf(&b, " %s %d) %-16s %-18s %d variant(s)\n",
			marker, i+1, th.Slug, th.Name, len(th.Variants))
	}
	if current != "" {
		fmt.Fprintf(&b, "\nCurrent: %s\n", current)
	} else {
		b.WriteString("\nCurrent: (none)\n")
	}
	return b.String()
}

// RenderVariantList formats a theme's variants, marking the current one.
func RenderVariantList(th theme.Theme, current string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Variants of %s:\n", th.Name)
	for i, v := range th.Variants {
		marker := " "
		if v.ID == current {
			marker = "*"
		}
		style := v.Style
		if style == "" {
			style = "-"
		}
		fmt.Fprintf(&b, " %s %d) %-12s %-14s [%s]\n", marker, i+1, v.ID, v.Name, style)
	}
	return b.String()
}

func readSelection(reader *bufio.Reader, n int) (int, error) {
	line, _ := reader.ReadString('\n')
	return ParseSelection(strings.TrimSpace(line), n)
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
    lumos                         Interactively pick a theme (and variant) to apply
    lumos <name> [variant]        Apply a theme; <name>/<variant> also works
    lumos --list                  List available themes and the current one
    lumos --install <src>         Install from a github repo, folder or .zip bundle
    lumos --install <src> --enable    Install and immediately apply it
    lumos --update [name]         Update one theme, or all themes when omitted

FLAGS:
    --enable        With --install, apply the theme after installing
    --dry-run, -n   Show what would change without writing files
    --list,    -l   Non-interactive theme listing
    --version       Print version
    --help,    -h   Show this help

A theme is a <name>.zip bundle: a theme.yaml manifest with one or more
variants (each a colour palette) plus a programs/ folder where every file is
one program's config. When a theme has several variants lumos asks which one
to apply; with a single variant it is used automatically.

Themes live in $XDG_CONFIG_HOME/lumos/themes (default ~/.config/lumos/themes).
Drop your own <name>.zip bundles there to have lumos manage them.

CUSTOM PORTS:
    For a program lumos doesn't know yet, add it to
    $XDG_CONFIG_HOME/lumos/ports.toml (default ~/.config/lumos/ports.toml).
    Each entry tells lumos where the theme file goes and what to run afterwards:

        [ports.cava]
        name = "cava"
        target = "${XDG_CONFIG_HOME}/cava/themes/${slug}-${variant}.conf"
        post = ["pkill -USR2 cava"]

    Custom ports add new programs and may override built-in ones. A theme's
    program file (e.g. programs/cava.conf) is matched to a port by its name.
`

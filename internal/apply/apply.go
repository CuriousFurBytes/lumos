// Package apply renders a theme variant against the port registry and
// installs the resulting files, running any reload hooks.
package apply

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/registry"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

// ResolvedProgram is a theme program rendered for a specific variant, with
// its destination path resolved to an absolute location.
type ResolvedProgram struct {
	Name    string
	Target  string
	Content string
	Post    []string
	Known   bool
}

// Runner executes a reload hook.
type Runner interface {
	Run(cmd string) error
}

// ExecRunner runs hooks through the system shell.
type ExecRunner struct{}

// Run executes cmd via `sh -c`.
func (ExecRunner) Run(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

var colorToken = regexp.MustCompile(`\$\{color\.([a-zA-Z0-9_-]+)\}`)

// Render turns a theme's discovered program files into installable units for
// variant v: ${color.KEY} tokens are replaced from the variant palette, and
// each program's destination and reload hooks come from the registry (keyed
// by the program's port). A program whose port is not in the registry is an
// error, since lumos cannot know where its file belongs.
func Render(t theme.Theme, v theme.Variant, paths config.Paths) ([]ResolvedProgram, error) {
	pathRepl := strings.NewReplacer(
		"${slug}", t.Slug,
		"${variant}", v.ID,
		"${name}", t.Name,
		"${variantName}", v.Name,
	)
	out := make([]ResolvedProgram, 0, len(t.Programs))
	for _, p := range t.Programs {
		port, known := registry.Lookup(p.Port)
		if !known {
			return nil, fmt.Errorf("program %q: unknown port, lumos has no install target for it", p.Port)
		}
		target := paths.Expand(pathRepl.Replace(port.Target))

		var rerr error
		content := colorToken.ReplaceAllStringFunc(p.Template, func(tok string) string {
			key := colorToken.FindStringSubmatch(tok)[1]
			val, ok := v.Colors[key]
			if !ok {
				rerr = fmt.Errorf("program %q: variant %q has no color %q", p.Port, v.ID, key)
			}
			return val
		})
		if rerr != nil {
			return nil, rerr
		}

		out = append(out, ResolvedProgram{
			Name: p.Port, Target: target, Content: content, Post: port.Post, Known: known,
		})
	}
	return out, nil
}

// Report summarises an apply run.
type Report struct {
	Applied  []string
	Hooks    []string
	Warnings []string
}

// Apply writes each rendered program to its target and runs its hooks. When
// dryRun is true nothing is written. Hook failures are recorded as warnings
// rather than aborting the switch.
func Apply(progs []ResolvedProgram, runner Runner, dryRun bool) (Report, error) {
	var rep Report
	for _, p := range progs {
		if dryRun {
			rep.Applied = append(rep.Applied, p.Name)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p.Target), 0o755); err != nil {
			return rep, fmt.Errorf("%s: %w", p.Name, err)
		}
		if err := os.WriteFile(p.Target, []byte(p.Content), 0o644); err != nil {
			return rep, fmt.Errorf("%s: %w", p.Name, err)
		}
		rep.Applied = append(rep.Applied, p.Name)
		for _, cmd := range p.Post {
			// Reload hooks are best-effort: a missing tool must not abort
			// the whole theme switch.
			if err := runner.Run(cmd); err != nil {
				rep.Warnings = append(rep.Warnings,
					fmt.Sprintf("%s: reload hook %q failed: %v", p.Name, cmd, err))
				continue
			}
			rep.Hooks = append(rep.Hooks, cmd)
		}
	}
	return rep, nil
}

// Package apply resolves a theme's programs against the port registry and
// installs the theme files onto disk, running any post-install hooks.
package apply

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/registry"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

// ResolvedProgram is a theme program with its source and destination paths
// fully resolved to absolute filesystem locations.
type ResolvedProgram struct {
	Name   string
	Source string // absolute path to the file inside the theme bundle
	Target string // absolute destination path
	Post   []string
	// Known reports whether the port was found in the registry.
	Known bool
}

// Runner executes a post-install shell command.
type Runner interface {
	Run(cmd string) error
}

// ExecRunner runs commands through the system shell.
type ExecRunner struct{}

// Run executes cmd via `sh -c`.
func (ExecRunner) Run(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Resolve turns a theme's declared programs into installable units. Targets
// and post-commands are taken from the theme when present, otherwise from
// the embedded registry. An unknown port with no explicit target is an
// error, since lumos cannot know where to put the file.
func Resolve(t theme.Theme, paths config.Paths) ([]ResolvedProgram, error) {
	repl := strings.NewReplacer("${slug}", t.Slug, "${name}", t.Name)
	out := make([]ResolvedProgram, 0, len(t.Programs))
	for _, p := range t.Programs {
		port, known := registry.Lookup(p.Name)

		target := p.Target
		if target == "" {
			target = port.Target
		}
		if target == "" {
			return nil, fmt.Errorf("program %q: unknown port and no target given", p.Name)
		}
		target = paths.Expand(repl.Replace(target))

		post := p.Post
		if post == nil {
			post = port.Post
		}

		out = append(out, ResolvedProgram{
			Name:   p.Name,
			Source: filepath.Join(t.Dir, p.File),
			Target: target,
			Post:   post,
			Known:  known,
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

// Apply installs each resolved program: it copies the source theme file to
// its target (creating parent directories) and runs post-install hooks.
// When dryRun is true it performs validation only and touches nothing.
func Apply(progs []ResolvedProgram, runner Runner, dryRun bool) (Report, error) {
	var rep Report
	for _, p := range progs {
		if _, err := os.Stat(p.Source); err != nil {
			return rep, fmt.Errorf("%s: theme file %s: %w", p.Name, p.Source, err)
		}
		if dryRun {
			rep.Applied = append(rep.Applied, p.Name)
			continue
		}
		if err := copyFile(p.Source, p.Target); err != nil {
			return rep, fmt.Errorf("%s: %w", p.Name, err)
		}
		rep.Applied = append(rep.Applied, p.Name)
		for _, cmd := range p.Post {
			// Reload/cache hooks are best-effort: a missing tool must not
			// abort the whole theme switch, so failures are warnings.
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

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

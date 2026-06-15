package builtin

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/theme"
)

func TestNamesIncludesStarterThemes(t *testing.T) {
	names := Names()
	want := map[string]bool{"catppuccin": false, "dracula": false, "rose-pine": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("builtin theme %q missing", n)
		}
	}
}

// expandedThemes are the default themes added in issue #7, on top of the
// original catppuccin/dracula/rose-pine trio.
var expandedThemes = []string{
	"monokai",
	"tokyo-night",
	"nord",
	"gruvbox",
	"one",
	"solarized",
	"aquadrive",
	"kanagawa",
	"synthwave-84",
	"material",
	"everforest",
	"poimandres",
	"night-owl",
	"adwaita",
	"ayu",
	"cobalt",
	"kanso",
	"flexoki",
}

func TestNamesIncludesExpandedThemes(t *testing.T) {
	have := make(map[string]bool)
	for _, n := range Names() {
		have[n] = true
	}
	for _, want := range expandedThemes {
		if !have[want] {
			t.Errorf("expanded builtin theme %q missing", want)
		}
	}
}

// TestExpandedThemesAreLoadable seeds every bundle and verifies each newly
// requested theme loads, exposes at least one variant, and ships program
// files. This is the core acceptance check for issue #7.
func TestExpandedThemesAreLoadable(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	for _, slug := range expandedThemes {
		th, err := theme.Load(filepath.Join(dir, slug+".zip"))
		if err != nil {
			t.Errorf("loading %q: %v", slug, err)
			continue
		}
		if len(th.Variants) == 0 {
			t.Errorf("%q has no variants", slug)
		}
		if len(th.Programs) == 0 {
			t.Errorf("%q has no programs", slug)
		}
		if th.Name == "" {
			t.Errorf("%q has empty name", slug)
		}
	}
}

// TestOneThemeHasLightAndDarkVariants verifies the One Dark/One Light request
// from issue #7 is satisfied by a single "one" bundle with both flavours.
func TestOneThemeHasLightAndDarkVariants(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(filepath.Join(dir, "one.zip"))
	if err != nil {
		t.Fatal(err)
	}
	var haveDark, haveLight bool
	for _, v := range th.Variants {
		switch v.Style {
		case "dark":
			haveDark = true
		case "light":
			haveLight = true
		}
	}
	if !haveDark {
		t.Error("one theme missing a dark variant (One Dark)")
	}
	if !haveLight {
		t.Error("one theme missing a light variant (One Light)")
	}
}

func TestSeedProducesLoadableZips(t *testing.T) {
	dir := t.TempDir()
	seeded, err := Seed(dir)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(seeded) == 0 {
		t.Fatal("nothing seeded")
	}
	// Every seeded bundle must be a real .zip that loads and has variants.
	themes, err := theme.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(themes) != len(seeded) {
		t.Errorf("discovered %d, seeded %d", len(themes), len(seeded))
	}
	for _, th := range themes {
		if len(th.Variants) == 0 {
			t.Errorf("%s has no variants", th.Slug)
		}
		if len(th.Programs) == 0 {
			t.Errorf("%s has no programs", th.Slug)
		}
	}
}

func TestCatppuccinHasFourVariants(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(filepath.Join(dir, "catppuccin.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if len(th.Variants) != 4 {
		t.Errorf("catppuccin variants = %d, want 4", len(th.Variants))
	}
}

var colorTokenRE = regexp.MustCompile(`\$\{color\.([a-zA-Z0-9_-]+)\}`)

// TestAllBuiltinThemesHaveCompletePalettes guards palette completeness: every
// ${color.KEY} token used by a theme's program files must be defined in every
// one of that theme's variants, for all builtin themes. apply.Render errors at
// runtime on a missing key, so verifying it statically here keeps every bundle
// internally consistent and safe to apply for any variant.
func TestAllBuiltinThemesHaveCompletePalettes(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	themes, err := theme.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, th := range themes {
		// Collect every color key referenced across the theme's programs.
		keys := map[string]bool{}
		for _, p := range th.Programs {
			for _, m := range colorTokenRE.FindAllStringSubmatch(p.Template, -1) {
				keys[m[1]] = true
			}
		}
		for _, v := range th.Variants {
			for key := range keys {
				if _, ok := v.Colors[key]; !ok {
					t.Errorf("%s/%s: program references undefined color %q", th.Slug, v.ID, key)
				}
			}
		}
	}
}

func TestSeedSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	if _, err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "dracula.zip")
	if err := os.WriteFile(existing, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}
	seeded, err := Seed(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range seeded {
		if s == "dracula" {
			t.Fatal("Seed overwrote an existing theme")
		}
	}
	data, _ := os.ReadFile(existing)
	if string(data) != "user data" {
		t.Error("existing theme content changed")
	}
}

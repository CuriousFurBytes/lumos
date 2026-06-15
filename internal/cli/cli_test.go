package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/apply"
	"github.com/CuriousFurBytes/lumos/internal/config"
	"github.com/CuriousFurBytes/lumos/internal/source"
	"github.com/CuriousFurBytes/lumos/internal/theme"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args []string
		want Options
	}{
		{nil, Options{Mode: ModeInteractive}},
		{[]string{"--list"}, Options{Mode: ModeList}},
		{[]string{"--install", "foo/bar"}, Options{Mode: ModeInstall, Spec: "foo/bar"}},
		{[]string{"--install", "foo/bar", "--enable"}, Options{Mode: ModeInstall, Spec: "foo/bar", Enable: true}},
		{[]string{"--update"}, Options{Mode: ModeUpdate}},
		{[]string{"--update", "dracula"}, Options{Mode: ModeUpdate, Name: "dracula"}},
		{[]string{"dracula"}, Options{Mode: ModeEnable, Name: "dracula"}},
		{[]string{"catppuccin", "mocha"}, Options{Mode: ModeEnable, Name: "catppuccin", Variant: "mocha"}},
		{[]string{"catppuccin/mocha"}, Options{Mode: ModeEnable, Name: "catppuccin", Variant: "mocha"}},
		{[]string{"--enable", "dracula"}, Options{Mode: ModeEnable, Name: "dracula"}},
		{[]string{"--help"}, Options{Mode: ModeHelp}},
		{[]string{"--version"}, Options{Mode: ModeVersion}},
		{[]string{"--dry-run", "dracula"}, Options{Mode: ModeEnable, Name: "dracula", DryRun: true}},
	}
	for _, c := range cases {
		got, err := ParseArgs(c.args)
		if err != nil {
			t.Errorf("ParseArgs(%v) error: %v", c.args, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseArgs(%v) = %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestParseArgsErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--install"}, // missing spec
		{"--bogus"},   // unknown flag
	} {
		if _, err := ParseArgs(args); err == nil {
			t.Errorf("ParseArgs(%v) expected error", args)
		}
	}
}

func TestRenderThemeList(t *testing.T) {
	themes := []theme.Theme{
		{Slug: "catppuccin", Name: "Catppuccin", Variants: []theme.Variant{{ID: "latte"}, {ID: "mocha"}}},
		{Slug: "dracula", Name: "Dracula", Variants: []theme.Variant{{ID: "dark"}}},
	}
	out := RenderThemeList(themes, "dracula")
	if !strings.Contains(out, "1)") || !strings.Contains(out, "2)") {
		t.Errorf("missing numbering:\n%s", out)
	}
	if !strings.Contains(out, "Catppuccin") || !strings.Contains(out, "Dracula") {
		t.Errorf("missing names:\n%s", out)
	}
	var currentLine string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "Dracula") {
			currentLine = l
		}
	}
	if !strings.Contains(currentLine, "*") {
		t.Errorf("current theme not marked:\n%s", out)
	}
}

func TestRenderVariantList(t *testing.T) {
	th := theme.Theme{Name: "Catppuccin", Variants: []theme.Variant{
		{ID: "latte", Name: "Latte", Style: "light"},
		{ID: "mocha", Name: "Mocha", Style: "dark"},
	}}
	out := RenderVariantList(th, "mocha")
	if !strings.Contains(out, "Latte") || !strings.Contains(out, "Mocha") {
		t.Errorf("missing variant names:\n%s", out)
	}
	if !strings.Contains(out, "light") || !strings.Contains(out, "dark") {
		t.Errorf("missing styles:\n%s", out)
	}
}

func TestParseSelection(t *testing.T) {
	if i, err := ParseSelection("2", 3); err != nil || i != 1 {
		t.Errorf("ParseSelection(2,3) = %d,%v", i, err)
	}
	if _, err := ParseSelection("", 3); err == nil {
		t.Error("empty selection should error (cancel)")
	}
	if _, err := ParseSelection("0", 3); err == nil {
		t.Error("0 should be out of range")
	}
	if _, err := ParseSelection("4", 3); err == nil {
		t.Error("4 should be out of range")
	}
	if _, err := ParseSelection("x", 3); err == nil {
		t.Error("non-number should error")
	}
}

// --- integration ---

func newTestApp(t *testing.T, in string) (*App, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))

	out := &bytes.Buffer{}
	app := &App{
		Paths:  config.Resolve(),
		Runner: noopRunner{},
		In:     strings.NewReader(in),
		Out:    out,
		Err:    out,
	}
	return app, out
}

type noopRunner struct{}

func (noopRunner) Run(string) error { return nil }

func TestInteractiveSelectThemeThenVariant(t *testing.T) {
	// Theme #1 (catppuccin) has 4 variants, so a second prompt appears.
	app, out := newTestApp(t, "1\n4\n")
	if code := app.Run(nil); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	st, err := config.LoadState(app.Paths.StateFile())
	if err != nil {
		t.Fatal(err)
	}
	if st.Current != "catppuccin" || st.Variant != "mocha" {
		t.Fatalf("state = %+v, want catppuccin/mocha", st)
	}
	th, err := theme.Load(filepath.Join(app.Paths.ThemesDir(), "catppuccin.zip"))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := th.Variant("mocha")
	progs, err := apply.Render(th, v, app.Paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progs {
		if !fileExists(p.Target) {
			t.Errorf("expected %s written for %s", p.Target, p.Name)
		}
	}
}

func TestEnableSingleVariantSkipsPrompt(t *testing.T) {
	// Dracula has one variant; no prompt is needed even with empty stdin.
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"dracula"}); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	st, _ := config.LoadState(app.Paths.StateFile())
	if st.Current != "dracula" || st.Variant != "dark" {
		t.Errorf("state = %+v, want dracula/dark", st)
	}
}

func TestEnableWithExplicitVariant(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"catppuccin", "frappe"}); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	st, _ := config.LoadState(app.Paths.StateFile())
	if st.Variant != "frappe" {
		t.Errorf("variant = %q, want frappe", st.Variant)
	}
}

func TestEnableUnknownVariantFails(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"catppuccin", "nope"}); code == 0 {
		t.Fatalf("expected failure for unknown variant\n%s", out.String())
	}
}

func TestEnableUnknownThemeFails(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"nope"}); code == 0 {
		t.Fatalf("expected failure enabling unknown theme\n%s", out.String())
	}
}

func TestInstallAndEnableFlow(t *testing.T) {
	app, out := newTestApp(t, "")
	app.Cloner = fakeCloner{}

	// A bundle directory: manifest + a program file under programs/.
	bundle := filepath.Join(t.TempDir(), "mine")
	writeFile(t, filepath.Join(bundle, "theme.yaml"),
		"name: Mine\nvariants:\n  - {id: only, name: Only, colors: {base: \"#000000\"}}\n")
	writeFile(t, filepath.Join(bundle, "programs", "alacritty.toml"), "bg = \"${color.base}\"")

	if code := app.Run([]string{"--install", bundle, "--enable"}); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if !fileExists(filepath.Join(app.Paths.ThemesDir(), "mine.zip")) {
		t.Error("theme not installed as a zip")
	}
	st, _ := config.LoadState(app.Paths.StateFile())
	if st.Current != "mine" || st.Variant != "only" {
		t.Errorf("install --enable should select theme, state=%+v", st)
	}
}

func TestListMode(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"--list"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "Catppuccin") {
		t.Errorf("list missing seeded themes:\n%s", out.String())
	}
}

// fakeCloner satisfies source.Cloner without network access.
type fakeCloner struct{}

func (fakeCloner) Clone(url, dest string) error { return nil }

var _ source.Cloner = fakeCloner{}

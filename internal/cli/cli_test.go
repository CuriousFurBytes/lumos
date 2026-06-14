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
		{Slug: "catppuccin-mocha", Name: "Catppuccin Mocha", Programs: []theme.Program{{Name: "a"}, {Name: "b"}}},
		{Slug: "dracula", Name: "Dracula", Programs: []theme.Program{{Name: "a"}}},
	}
	out := RenderThemeList(themes, "dracula")
	if !strings.Contains(out, "1)") || !strings.Contains(out, "2)") {
		t.Errorf("missing numbering:\n%s", out)
	}
	if !strings.Contains(out, "Catppuccin Mocha") || !strings.Contains(out, "Dracula") {
		t.Errorf("missing names:\n%s", out)
	}
	// The current theme must be marked.
	lineWithCurrent := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "Dracula") {
			lineWithCurrent = l
		}
	}
	if !strings.Contains(lineWithCurrent, "*") {
		t.Errorf("current theme not marked:\n%s", out)
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
		Cloner: nil,
		In:     strings.NewReader(in),
		Out:    out,
		Err:    out,
	}
	return app, out
}

type noopRunner struct{}

func (noopRunner) Run(string) error { return nil }

func TestInteractiveSelectAppliesAndPersists(t *testing.T) {
	app, out := newTestApp(t, "1\n")
	// Seeds builtin themes, lists them, applies selection #1.
	if code := app.Run(nil); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	st, err := config.LoadState(app.Paths.StateFile())
	if err != nil {
		t.Fatal(err)
	}
	if st.Current == "" {
		t.Fatal("no theme persisted after selection")
	}
	// The selected theme's files must have been written somewhere in config.
	themes, _ := theme.Discover(app.Paths.ThemesDir())
	var chosen theme.Theme
	for _, th := range themes {
		if th.Slug == st.Current {
			chosen = th
		}
	}
	progs, err := apply.Resolve(chosen, app.Paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progs {
		if !fileExists(p.Target) {
			t.Errorf("expected %s written for %s", p.Target, p.Name)
		}
	}
}

func TestEnableByName(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"dracula"}); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	st, _ := config.LoadState(app.Paths.StateFile())
	if st.Current != "dracula" {
		t.Errorf("current = %q, want dracula", st.Current)
	}
}

func TestEnableUnknownNameFails(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"nope"}); code == 0 {
		t.Fatalf("expected failure enabling unknown theme\n%s", out.String())
	}
}

func TestInstallAndEnableFlow(t *testing.T) {
	app, out := newTestApp(t, "")
	app.Cloner = &fakeCloner{}

	// Build a local theme folder to install.
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "theme.toml"),
		"name=\"My Theme\"\nslug=\"mine\"\n[[programs]]\nname=\"alacritty\"\nfile=\"a.toml\"\n")
	writeFile(t, filepath.Join(src, "a.toml"), "# x")

	if code := app.Run([]string{"--install", src, "--enable"}); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if !fileExists(filepath.Join(app.Paths.ThemesDir(), "mine", "theme.toml")) {
		t.Error("theme not installed")
	}
	st, _ := config.LoadState(app.Paths.StateFile())
	if st.Current != "mine" {
		t.Errorf("install --enable should select theme, current=%q", st.Current)
	}
}

func TestListMode(t *testing.T) {
	app, out := newTestApp(t, "")
	if code := app.Run([]string{"--list"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "Catppuccin Mocha") {
		t.Errorf("list missing seeded themes:\n%s", out.String())
	}
}

// fakeCloner satisfies source.Cloner without network access.
type fakeCloner struct{}

func (fakeCloner) Clone(url, dest string) error { return nil }
func (fakeCloner) Pull(dir string) error        { return nil }

var _ source.Cloner = fakeCloner{}

# lumos

> _"Lumos."_ — light up every program at once.

**lumos** is a small Go CLI that switches colour themes across *all* your
programs in one shot. Pick a theme and a variant once, and lumos renders the
right theme file into the right place for every program that theme supports —
terminals, editors, CLI tools, launchers, notification daemons and more.

A theme is a single [YAML 1.2](https://yaml.org/spec/1.2.2/) file. It declares
its programs once as templates and provides one or more **variants** (e.g.
light/dark flavours), each with its own colour palette. lumos fills the
palette into the templates for whichever variant you pick, so one file themes
every supported program for every variant.

lumos ships with a **port base** seeded from the canonical port lists of the
major theming projects — [Catppuccin](https://github.com/catppuccin/catppuccin),
[Dracula](https://github.com/dracula),
[Base16 & Base24](https://github.com/tinted-theming) and
[Rosé Pine](https://github.com/rose-pine) — so theme authors rarely have to
say *where* a program's theme file goes; lumos already knows.

---

## Install

```sh
go install github.com/CuriousFurBytes/lumos@latest
```

This drops a `lumos` binary in `$(go env GOPATH)/bin` (make sure that's on your
`PATH`). lumos is built with **Go 1.26**.

Build from source instead:

```sh
git clone https://github.com/CuriousFurBytes/lumos
cd lumos
make build      # produces ./lumos
```

---

## Usage

### Pick a theme interactively

```sh
lumos
```

lumos lists the available themes, marks the one currently selected, and applies
your choice. If the chosen theme has several variants it then asks which
variant; a theme with a single variant is applied immediately.

```
Available themes:
 * 1) catppuccin       Catppuccin         4 variant(s)
   2) dracula          Dracula            1 variant(s)
   3) rose-pine        Rosé Pine          3 variant(s)

Current: catppuccin

Select a theme number (enter to cancel): 3

Variants of Rosé Pine:
   1) main         Main           [dark]
   2) moon         Moon           [dark]
   3) dawn         Dawn           [light]

Select a variant for Rosé Pine (enter to cancel): 3
Applied Rosé Pine (Dawn) to 2 program(s): alacritty, kitty
```

### Apply a theme directly

```sh
lumos dracula                # single-variant theme: applied straight away
lumos catppuccin mocha       # theme + variant
lumos catppuccin/mocha       # same, slash form
lumos --list                 # list themes without the prompt
lumos --dry-run rose-pine/dawn   # show what would change, write nothing
```

If you name a multi-variant theme without a variant, lumos prompts for one.

### Install a theme

From a GitHub repository (shorthand or full URL), a local folder, or a single
`.yaml` file:

```sh
lumos --install CuriousFurBytes/my-themes      # github owner/repo shorthand
lumos --install https://github.com/foo/bar     # full URL
lumos --install ./catppuccin.yaml              # a single theme file
lumos --install ./my-themes/                   # a folder of theme files

# install and switch to it immediately
lumos --install CuriousFurBytes/my-themes --enable
```

A repository or folder may contain any number of theme `.yaml`/`.yml` files;
lumos installs all of them.

### Update themes

```sh
lumos --update            # update every installed theme from its source
lumos --update dracula    # update just one
```

Updates re-fetch from wherever the theme was installed from (git clone for
repositories, re-copy for local sources).

### All flags

```
lumos                         Interactively pick a theme (and variant)
lumos <name> [variant]        Apply a theme; <name>/<variant> also works
lumos --list, -l              List available themes and the current one
lumos --install <src>         Install from a github repo, folder or .yaml file
lumos --install <src> --enable    Install and immediately apply
lumos --update [name]         Update one theme, or all when omitted
lumos --dry-run, -n           Show what would change without writing
lumos --version
lumos --help, -h
```

---

## Where things live

lumos follows the XDG base directory spec:

| What                 | Location (default)                                        |
| -------------------- | --------------------------------------------------------- |
| Themes               | `$XDG_CONFIG_HOME/lumos/themes` (`~/.config/lumos/themes`) |
| Selected state       | `$XDG_STATE_HOME/lumos/state.toml`                        |

**Custom themes** are just `.yaml` files dropped into
`$XDG_CONFIG_HOME/lumos/themes/` — lumos manages them like any other. On first
run lumos seeds a few starter themes (Catppuccin, Dracula, Rosé Pine) so
there's something to switch to immediately; your own edits are never
overwritten.

---

## Theme file format

A theme is one YAML file. Programs are declared once; each variant supplies a
palette that fills the program templates.

```yaml
name: "Catppuccin"
family: catppuccin
source: "https://github.com/catppuccin/catppuccin"   # used by --update
description: "Soothing pastel theme for the high-spirited."

# slug defaults to the file name (catppuccin.yaml -> "catppuccin").

programs:
  - name: alacritty            # a port key from the registry
    template: |                # ${color.X} is filled from the active variant
      [colors.primary]
      background = "${color.base}"
      foreground = "${color.text}"
    # target defaults to the registry entry; override if you need to:
    # target: "${XDG_CONFIG_HOME}/alacritty/themes/${slug}-${variant}.toml"

  - name: bat
    content: "…literal file, no palette…"   # use content instead of template
    post: ["bat cache --build"]             # reload hooks (best-effort)

variants:
  - id: latte                  # defaults to a slug of `name` when omitted
    name: "Latte"
    style: light
    colors:
      base: "#eff1f5"
      text: "#4c4f69"
  - id: mocha
    name: "Mocha"
    style: dark
    colors:
      base: "#1e1e2e"
      text: "#cdd6f4"
```

### Program fields

| Field      | Required | Meaning                                                       |
| ---------- | -------- | ------------------------------------------------------------- |
| `name`     | yes      | Port key. Looked up in the registry for the default `target`. |
| `template` | one of   | Rendered with the variant palette via `${color.KEY}` tokens.  |
| `content`  | one of   | Literal output, for programs that don't use the palette.      |
| `target`   | no       | Destination path. Defaults to the registry entry for `name`.  |
| `post`     | no       | Shell hooks run after writing (e.g. cache rebuilds).          |

### Variant fields

| Field    | Required | Meaning                                            |
| -------- | -------- | -------------------------------------------------- |
| `id`     | no       | Stable id (defaults to a slug of `name`).          |
| `name`   | yes      | Display name shown in the picker.                  |
| `style`  | no       | `light` / `dark`, shown in the variant list.       |
| `colors` | no\*     | Palette map; \*required if any `template` uses it. |

### Placeholders

`template` colours use `${color.KEY}`, looked up in the variant's `colors`.
Referencing a key the variant doesn't define is an error.

`target` paths understand:

- `${XDG_CONFIG_HOME}`, `${XDG_DATA_HOME}`, `${XDG_STATE_HOME}`, `${XDG_CACHE_HOME}`
- `$HOME`, `~`
- `${slug}` (theme slug), `${variant}` (variant id)
- `${name}` (theme name), `${variantName}` (variant name)

Post-install hooks are **best-effort**: if a program isn't installed (so its
reload command fails) lumos warns and carries on rather than aborting the
switch.

---

## The port base

`lumos` embeds a base of known programs ("ports") in
[`internal/registry/ports.toml`](internal/registry/ports.toml), seeded from the
upstream theming projects' published port lists. Each entry records where that
program conventionally expects its theme file, so a theme can simply say
`name: kitty` and lumos knows to write to
`~/.config/kitty/themes/<slug>-<variant>.conf`.

Covered programs include terminals (Alacritty, kitty, WezTerm, foot, Ghostty,
Rio), CLI tools (bat, delta, btop, bottom, fzf, lazygit, k9s, yazi, gitui),
editors (Helix, Neovim, Vim, micro), shells/prompts (fish, starship) and
desktop bits (rofi, dunst, mako, waybar, Hyprland, sway, i3, polybar, GTK) —
among others. A theme can target any program by giving an explicit `target`,
even one not in the base.

---

## Development

```sh
make test     # go test -race ./...
make check    # gofmt check + go vet + staticcheck + tests (the CI gate)
make build    # build ./lumos
```

The project is built with **red-green TDD** — every package has a test suite
written before its implementation. CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml))
runs the tests with the race detector, checks formatting (`gofmt`), runs
`go vet` and `staticcheck`, and verifies that `go install` produces a working
binary.

### Layout

```
main.go                     thin entrypoint
internal/cli                arg parsing, interactive picker, command dispatch
internal/theme              theme file model + YAML loading/discovery
internal/registry           embedded port base (ports.toml)
internal/apply              render a variant against the registry and install files
internal/source             install/update from git repos, folders or files
internal/config             XDG paths + selected-theme state
internal/builtin            embedded starter themes, seeded on first run
```

## License

See [LICENSE](LICENSE).

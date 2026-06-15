# lumos

> _"Lumos."_ — light up every program at once.

**lumos** is a small Go CLI that switches colour themes across *all* your
programs in one shot. Pick a theme and a variant once, and lumos renders the
right theme file into the right place for every program that theme supports —
terminals, editors, CLI tools, launchers, notification daemons and more.

A theme is a `<name>.zip` bundle: a [YAML 1.2](https://yaml.org/spec/1.2.2/)
manifest with the theme's metadata and one or more **variants** (e.g.
light/dark flavours, each a colour palette), plus a `programs/` folder where
**every file is one program's config**. lumos fills the active variant's
palette into those files and installs each one where its program expects it.

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
`.zip` bundle:

```sh
lumos --install CuriousFurBytes/my-themes      # github owner/repo shorthand
lumos --install https://github.com/foo/bar     # full URL
lumos --install ./catppuccin.zip               # a packed bundle
lumos --install ./catppuccin/                  # an unpacked bundle directory

# install and switch to it immediately
lumos --install CuriousFurBytes/my-themes --enable
```

A repository or folder may contain any number of bundles (packed `.zip`s or
bundle directories); lumos installs all of them as `<slug>.zip`.

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
lumos --install <src>         Install from a github repo, folder or .zip bundle
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

**Custom themes** are `<name>.zip` bundles dropped into
`$XDG_CONFIG_HOME/lumos/themes/` — lumos manages them like any other (a plain
bundle directory works too). On first run lumos seeds a few starter themes
(Catppuccin, Dracula, Rosé Pine) as zips so there's something to switch to
immediately; your own files are never overwritten.

---

## Theme bundle format

A theme is a `<name>.zip` whose contents are:

```
catppuccin.zip
├── theme.yaml          # metadata + variants (palettes) — NOT a list of programs
└── programs/
    ├── alacritty.toml  # one file per program; the file name is the port key
    ├── kitty.conf      # any ${color.X} is filled from the active variant
    └── …
```

The manifest only describes the theme and its variants:

```yaml
# theme.yaml
name: "Catppuccin"
family: catppuccin
source: "https://github.com/catppuccin/catppuccin"   # used by --update
description: "Soothing pastel theme for the high-spirited."

# slug defaults to the bundle name (catppuccin.zip -> "catppuccin").

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

Each file under `programs/` is one program's config. Its name minus extension
is the **port key**, which lumos looks up in the registry to know where to
install it. The file body may use `${color.KEY}` tokens:

```toml
# programs/alacritty.toml  ->  port "alacritty"
[colors.primary]
background = "${color.base}"
foreground = "${color.text}"
```

A file with no tokens is installed verbatim for every variant.

### Variant fields

| Field    | Required | Meaning                                            |
| -------- | -------- | -------------------------------------------------- |
| `id`     | no       | Stable id (defaults to a slug of `name`).          |
| `name`   | yes      | Display name shown in the picker.                  |
| `style`  | no       | `light` / `dark`, shown in the variant list.       |
| `colors` | no\*     | Palette map; \*required if any program file uses a token from it. |

### Authoring & rules

- A program file's port must exist in the registry (that's how lumos knows the
  destination); referencing a `${color.KEY}` a variant doesn't define is an
  error.
- During development you can keep a bundle as a **plain directory** (same
  layout) — `lumos --install ./mytheme/` zips it for you, and `lumos` reads
  either form.
- Reload hooks (e.g. `bat cache --build`) come from the registry entry and are
  **best-effort**: if a program isn't installed, lumos warns and carries on.

---

## The port base

`lumos` embeds a base of known programs ("ports") in
[`internal/registry/ports.toml`](internal/registry/ports.toml), seeded from the
upstream theming projects' published port lists. Each entry records where that
program conventionally expects its theme file (and any reload hook), so a file
named `programs/kitty.conf` is installed to
`~/.config/kitty/themes/<slug>-<variant>.conf` automatically. The destination
templates understand `${slug}`, `${variant}`, `${name}`, `${variantName}` and
the XDG/`$HOME`/`~` placeholders.

Covered programs include terminals (Alacritty, kitty, WezTerm, foot, Ghostty,
Rio), CLI tools (bat, delta, btop, bottom, fzf, lazygit, k9s, yazi, gitui),
editors (Helix, Neovim, Vim, micro), shells/prompts (fish, starship) and
desktop bits (rofi, dunst, mako, waybar, Hyprland, sway, i3, polybar, GTK) —
among others. To support a program lumos doesn't know yet, add it to the port
base.

---

## Versioning & releases

lumos uses [SemVer 2.0.0](https://semver.org). While it is pre-1.0, every
release is an **alpha pre-release**, starting at `v0.0.1-alpha.1`. Pre-release
tags use the canonical hyphenated form (`vX.Y.Z-alpha.N`), so `go install
…@latest` keeps installing the latest *stable* release and ignores alphas —
install a specific alpha explicitly:

```sh
go install github.com/CuriousFurBytes/lumos@v0.0.1-alpha.1
```

`lumos --version` reports the tag it was built from (via build info, or the
`-ldflags` value used by the release build).

### Automatic tags on merge

Every merge to `main` runs
[`.github/workflows/release.yml`](.github/workflows/release.yml), which:

1. finds the latest `v*` tag,
2. computes the next version (an alpha bump by default — see below),
3. creates and pushes that tag, and
4. publishes a GitHub Release with cross-platform binaries (via
   [GoReleaser](https://goreleaser.com); `-alpha` tags are marked as
   pre-releases automatically).

The bump rules live in the tested [`internal/version`](internal/version)
package and are driven by the `tools/nextver` helper:

| Situation / input            | Example result                       |
| ---------------------------- | ------------------------------------ |
| no tags yet                  | `v0.0.1-alpha.1`                     |
| default merge (alpha bump)   | `…-alpha.1` → `…-alpha.2`            |
| commit contains `[patch]`    | `v0.0.1-alpha.3` → `v0.0.2-alpha.1` |
| commit contains `[minor]`    | `v0.3.4-alpha.5` → `v0.4.0-alpha.1` |
| commit contains `[major]`    | `v0.3.4-alpha.5` → `v1.0.0-alpha.1` |
| commit contains `[stable]`   | `v0.3.4-alpha.5` → `v0.3.4`         |

You can also cut a release on demand from the Actions tab via **Run workflow**,
choosing the bump level (`prerelease`/`patch`/`minor`/`major`/`stable`).

> The workflow needs the repository's Actions permission set to
> **Read and write** (Settings → Actions → General → Workflow permissions) so
> it can push tags and publish releases with the built-in `GITHUB_TOKEN`.

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
internal/theme              zip/dir bundle model + manifest loading/discovery
internal/registry           embedded port base (ports.toml)
internal/apply              render a variant against the registry and install files
internal/source             install/update bundles from repos, folders or zips
internal/config             XDG paths + selected-theme state
internal/builtin            embedded starter themes, seeded on first run
```

## License

See [LICENSE](LICENSE).

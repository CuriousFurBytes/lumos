# lumos

> _"Lumos."_ — light up every program at once.

**lumos** is a small Go CLI that switches colour themes across *all* your
programs in one shot. Pick a theme once and lumos writes the right theme file
into the right place for every program that theme supports — terminals,
editors, CLI tools, launchers, notification daemons and more.

Themes are plain [TOML](https://toml.io) bundles. Each bundle lists the
programs it touches and the theme file to install for each. lumos ships with a
**port base** seeded from the canonical port lists of the major theming
projects — [Catppuccin](https://github.com/catppuccin/catppuccin),
[Dracula](https://github.com/dracula),
[Base16 & Base24](https://github.com/tinted-theming) and
[Rosé Pine](https://github.com/rose-pine) — so theme authors rarely have to
specify *where* a program's theme file goes; lumos already knows.

---

## Install

```sh
go install github.com/CuriousFurBytes/lumos@latest
```

This drops a `lumos` binary in `$(go env GOPATH)/bin` (make sure that's on your
`PATH`). lumos requires **Go 1.26+** to build.

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

Shows the available themes, marks the one currently selected, and applies the
one you choose:

```
Available themes:
   1) catppuccin-mocha       Catppuccin Mocha (3 program(s))
 * 2) dracula                Dracula (2 program(s))
   3) rose-pine              Rose Pine (2 program(s))

Current: dracula

Select a theme number (enter to cancel):
```

### Apply a theme directly

```sh
lumos catppuccin-mocha      # apply by slug
lumos --list                # list themes without the prompt
lumos --dry-run rose-pine   # show what would change, write nothing
```

### Install a theme

From a GitHub repository (shorthand or full URL) or a local folder:

```sh
lumos --install CuriousFurBytes/my-themes      # github owner/repo shorthand
lumos --install https://github.com/foo/bar     # full URL
lumos --install ./my-local-theme               # local folder

# install and switch to it immediately
lumos --install CuriousFurBytes/my-themes --enable
```

A repository may contain a single theme (a `theme.toml` at its root) or many
(each in its own subdirectory, optionally under a `themes/` directory). lumos
installs all of them.

### Update themes

```sh
lumos --update            # update every installed theme from its source
lumos --update dracula    # update just one
```

Updates re-fetch from wherever the theme was installed from (git pull for
repositories, re-copy for local folders).

### All flags

```
lumos                         Interactively pick a theme to apply
lumos <name>                  Apply a theme by slug
lumos --list, -l              List available themes and the current one
lumos --install <src>         Install from a github repo or local folder
lumos --install <src> --enable    Install and immediately apply
lumos --update [name]         Update one theme, or all when omitted
lumos --dry-run, -n           Show what would change without writing
lumos --version
lumos --help, -h
```

---

## Where things live

lumos follows the XDG base directory spec:

| What                 | Location (default)                          |
| -------------------- | ------------------------------------------- |
| Themes               | `$XDG_CONFIG_HOME/lumos/themes` (`~/.config/lumos/themes`) |
| Selected-theme state | `$XDG_STATE_HOME/lumos/state.toml`          |

**Custom themes** go straight into `$XDG_CONFIG_HOME/lumos/themes/<slug>/` —
drop a bundle there and lumos manages it like any other. On first run lumos
seeds a few starter themes (Catppuccin Mocha, Dracula, Rosé Pine) so there's
something to switch to immediately; your own edits are never overwritten.

---

## Theme bundle format

A theme bundle is a directory containing a `theme.toml` manifest plus the
colour files it references:

```
catppuccin-mocha/
├── theme.toml
├── alacritty.toml
├── kitty.conf
└── bat.tmTheme
```

```toml
# theme.toml
name    = "Catppuccin Mocha"
family  = "catppuccin"
flavor  = "mocha"
source  = "https://github.com/catppuccin/catppuccin"   # used by --update
description = "Soothing pastel dark theme."

# slug defaults to the directory name; set it explicitly to override.

[[programs]]
name = "alacritty"          # a port key from the registry
file = "alacritty.toml"     # the file in this bundle to install
# target is filled in from the registry; override it if you need to:
# target = "${XDG_CONFIG_HOME}/alacritty/themes/${slug}.toml"

[[programs]]
name = "bat"
file = "bat.tmTheme"
post = ["bat cache --build"]   # reload/rebuild hooks (best-effort)
```

### Program fields

| Field         | Required | Meaning                                                            |
| ------------- | -------- | ------------------------------------------------------------------ |
| `name`        | yes      | Port key. Looked up in the registry to fill in `target`/`post`.    |
| `file`        | yes      | Theme file inside the bundle to install.                           |
| `target`      | no       | Destination path. Defaults to the registry entry for `name`.       |
| `post`        | no       | Shell commands to run after writing (e.g. cache rebuilds).         |
| `description` | no       | Human note.                                                        |

### Path placeholders

`target` paths understand:

- `${XDG_CONFIG_HOME}`, `${XDG_DATA_HOME}`, `${XDG_STATE_HOME}`, `${XDG_CACHE_HOME}`
- `$HOME`, `~`
- `${slug}` — the theme's slug
- `${name}` — the theme's display name

Post-install hooks are **best-effort**: if a program isn't installed (so its
reload command fails) lumos warns and carries on rather than aborting the whole
switch.

---

## The port base

`lumos` embeds a base of known programs ("ports") in
[`internal/registry/ports.toml`](internal/registry/ports.toml), seeded from the
upstream theming projects' published port lists. Each entry records where that
program conventionally expects its theme file, so a theme bundle can simply say
`name = "kitty"` and lumos knows to write to
`~/.config/kitty/themes/<slug>.conf`.

Covered programs include terminals (Alacritty, kitty, WezTerm, foot, Ghostty,
Rio), CLI tools (bat, delta, btop, bottom, fzf, lazygit, k9s, yazi, gitui),
editors (Helix, Neovim, Vim, micro), shells/prompts (fish, starship) and
desktop bits (rofi, dunst, mako, waybar, Hyprland, sway, i3, polybar, GTK) —
among others. Themes can target any program by giving an explicit `target`,
even if it isn't in the base.

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
internal/theme              theme bundle model + TOML loading
internal/registry           embedded port base (ports.toml)
internal/apply              resolve programs against the registry and install files
internal/source             install/update from git repos or local folders
internal/config             XDG paths + selected-theme state
internal/builtin            embedded starter themes, seeded on first run
```

## License

See [LICENSE](LICENSE).

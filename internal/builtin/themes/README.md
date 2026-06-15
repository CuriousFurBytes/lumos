# Lumos Themes Package

This directory contains all available themes for the Lumos global theme switcher. Each theme is organized as a separate package with metadata and port-specific configurations.

## Directory Structure

```
themes/
├── aquadrive/
│   ├── theme.yaml           # Theme metadata and color variants
│   └── ports/
│       ├── alacritty.toml   # Alacritty terminal configuration
│       └── kitty.conf       # Kitty terminal configuration
├── catppuccin/
│   ├── theme.yaml
│   └── ports/
│       ├── alacritty.toml
│       └── kitty.conf
└── [... other themes ...]
```

## Theme Metadata (theme.yaml)

Each theme must include a `theme.yaml` file with the following structure:

```yaml
name: "Theme Name"
family: theme-family
source: "https://github.com/source/url"
description: "Short description of the theme"

variants:
  - id: dark
    name: "Display Name"
    style: dark  # or 'light'
    colors:
      base: "#XXXXXX"      # Background color
      text: "#XXXXXX"      # Foreground color
      black: "#XXXXXX"
      red: "#XXXXXX"
      green: "#XXXXXX"
      yellow: "#XXXXXX"
      blue: "#XXXXXX"
      magenta: "#XXXXXX"
      cyan: "#XXXXXX"
      white: "#XXXXXX"
```

## Port Configurations

Port files use the following color variable placeholders:
- `${color.base}` or `${base}` - Background color
- `${color.text}` or `${text}` - Foreground color
- `${color.black}`, `${color.red}`, etc. - Standard color names

### Supported Ports

- **alacritty.toml** - Alacritty terminal emulator
- **kitty.conf** - Kitty terminal emulator

## Adding a New Theme

1. Create a new directory under `themes/{theme-slug}/`
2. Add `theme.yaml` with metadata and color variants
3. Create `ports/` subdirectory with terminal configurations
4. Use color placeholders in port configuration files

## Theme Sources

- **Aquadrive** - https://github.com/CuriousFurBytes/aquadrive
- **Catppuccin** - https://github.com/catppuccin/catppuccin
- **Dracula** - https://github.com/dracula/dracula-theme
- **Gruvbox** - https://github.com/gruvbox-community/gruvbox
- **Nord** - https://github.com/arcticicestudio/nord
- **Solarized** - https://github.com/altercation/solarized
- And many more community themes!

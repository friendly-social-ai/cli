# Theme

Put a theme at `~/.config/friendly/theme.toml`, or at `$XDG_CONFIG_HOME/friendly/theme.toml` when `XDG_CONFIG_HOME` is
set. `FRIENDLY_CONFIG_DIR` picks another folder, see [config.md](config.md#another-folder). List only the colors you
change. Everything else keeps its default. [config/theme.toml](../config/theme.toml) lists every color with its default,
ready to copy.

```toml
mode = "dark"            # auto, dark or light

[dark]
primary   = "#CBA6F7"
selection = "#313244"
```

## Rules

- `mode` picks the colors. `auto`, the default, asks the terminal whether its background is dark. `dark` and `light`
  skip the question, for terminals that answer wrong.
- `[dark]` holds colors for a dark background and `[light]` for a light one. Only the section the mode picks applies.
- A color is `#RRGGBB`, `#RGB`, or an ANSI color number from `0` to `255`. On a terminal with fewer colors, the app
  picks the closest one it supports.
- A file with mistakes stops the app before it starts and lists every mistake. Nothing from the file applies until you
  fix it.

## Colors

| Color | Used for | Dark | Light |
|---|---|---|---|
| `primary` | accent text, key hints, the selection marker, focused borders, links and headings | `#6E8BFF` | `#0060D0` |
| `muted` | secondary text, hints and link addresses | `#B4B4B4` | `#646464` |
| `danger` | errors and confirmations | `#F0715A` | `#C4391D` |
| `border` | lines under the header and over the footer, input borders and the help panel | `#4E4E4E` | `#C8C8C8` |
| `selection` | background of the selected item | `#232A3B` | `#E8EDF9` |

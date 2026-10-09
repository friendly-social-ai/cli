# Settings

Put settings at `~/.config/friendly/config.toml`, or at `$XDG_CONFIG_HOME/friendly/config.toml` when `XDG_CONFIG_HOME`
is set. `FRIENDLY_CONFIG_DIR` picks another folder, see [below](#another-folder). List only the settings you change.
Everything else keeps its default. [config/config.toml](../config/config.toml) lists every setting with its default,
ready to copy. Keys and colors have their own files, see [keys.md](keys.md) and [theme.md](theme.md).

```toml
images  = "off"
refresh = "5m"
```

## Settings

| Setting | Values | Default |
|---|---|---|
| `images` | `auto` draws images with terminal graphics in Ghostty and Kitty, and with colored blocks elsewhere. `graphics` skips the detection, for a terminal that supports them but isn't detected. `blocks` always uses colored blocks. `off` shows `[image]` and downloads nothing. The links key, `o` by default, still opens an image. Display math between `$$` lines draws as an image only with terminal graphics and [typst](https://typst.app) installed, and shows as Unicode otherwise. | `auto` |
| `refresh` | How often the app checks for new posts and activity, like `30s` or `5m`. The shortest is `30s`. `"0"` turns the checks off, and the refresh key, `R` by default, still works. Relative times like "5m ago" update every minute either way. | `1m` |

A file with mistakes stops the app before it starts and lists every mistake. Nothing from the file applies until you
fix it.

## Another folder

Set `FRIENDLY_CONFIG_DIR` to read `config.toml`, `keys.toml` and `theme.toml` from another folder, for example to try
a theme without touching your own files.

```bash
FRIENDLY_CONFIG_DIR=~/friendly-test friendly
```

The files go straight in that folder, without a `friendly` folder inside it. [config/](../config) in this repo is laid
out that way, so `FRIENDLY_CONFIG_DIR=config` works from the repo root. A path that isn't a folder stops the app, so a
typo can't leave you on the defaults without noticing.

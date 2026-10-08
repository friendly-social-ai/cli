# Settings

Put settings at `$XDG_CONFIG_HOME/friendly/config.toml`, or `~/.config/friendly/config.toml` when `XDG_CONFIG_HOME`
isn't set. List only the settings you change. Everything else keeps its default. [config.toml](config.toml) lists
every setting with its default, ready to copy. Keys and colors have their own files, see [keys.md](keys.md) and
[theme.md](theme.md).

```toml
images  = "off"
refresh = "5m"
```

## Settings

| Setting | Values | Default |
|---|---|---|
| `images` | `auto` draws images with terminal graphics in Ghostty and Kitty, and with colored blocks elsewhere. `graphics` skips the detection, for a terminal that supports them but isn't detected. `blocks` always uses colored blocks. `off` shows `[image]` and downloads nothing. The links key, `o` by default, still opens an image. | `auto` |
| `refresh` | How often the app checks for new posts and activity, like `30s` or `5m`. The shortest is `30s`. `"0"` turns the checks off, and the refresh key, `r` by default, still works. Relative times like "5m ago" update every minute either way. | `1m` |

A file with mistakes stops the app before it starts and lists every mistake. Nothing from the file applies until you
fix it.

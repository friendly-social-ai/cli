# Friendly CLI

A terminal client for Friendly, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

This is a fork of the [official CLI](https://github.com/friendly-social/cli). It runs on the forked
[Go SDK](https://github.com/friendly-social-ai/golang-sdk), pinned to its `main`.

## Installation

There's no official release for this project, you'll have to build from source and move it to your bin.

### Shameless plug

This project can be installed via [oku](https://github.com/y3owk1n/oku) with the following command:

```bash
oku add github:friendly-social-ai/cli
```

## Build

Requires Go 1.26.

```bash
git clone https://github.com/friendly-social-ai/cli.git
cd cli
make run
```

`make build` writes the binary to `bin/cli`.

## Nice supports

- Images render in Ghostty and Kitty, also inside tmux with `allow-passthrough` on. With a low resolution fallback.
- The composer can open a draft in `$VISUAL`, then `$EDITOR`, then `vi`.
- Pasting an image reads the clipboard with `osascript` on macOS, and `wl-paste` or `xclip` on Linux.
- Emoji shortcode supports
- Drag, paste or attach (with auto path completion) for images
- Change any key in `~/.config/friendly/keys.toml`, see [docs/keys.md](docs/keys.md)
- Change the colors in `~/.config/friendly/theme.toml`, see [docs/theme.md](docs/theme.md)
- Turn images off or change how often the app checks for new posts in `~/.config/friendly/config.toml`, see
  [docs/config.md](docs/config.md)
- Start with another config folder through `FRIENDLY_CONFIG_DIR`
- [config/](config) holds every default config file, ready to copy to `~/.config/friendly`

## Development

```bash
make fmt   # gofmt
make lint  # golangci-lint
```

Set `FRIENDLY_DEBUG=1` or `2` to write a debug log, see [docs/debug.md](docs/debug.md).

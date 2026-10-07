# Friendly CLI

A terminal client for Friendly, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

This is a fork of the [official CLI](https://github.com/friendly-social/cli). It runs on the forked
[Go SDK](https://github.com/friendly-social-ai/golang-sdk), pinned to its `main`.

## Build

Requires Go 1.26.

```bash
git clone https://github.com/friendly-social-ai/cli.git
cd cli
make run
```

`make build` writes the binary to `bin/cli`.

## Terminal support

- Images render in Ghostty and Kitty, also inside tmux with `allow-passthrough` on.
- The composer can open a draft in `$VISUAL`, then `$EDITOR`, then `vi`.
- Pasting an image reads the clipboard with `osascript` on macOS, and `wl-paste` or `xclip` on Linux.

## Development

```bash
make fmt   # gofmt
make lint  # golangci-lint
```

Set `DEBUG=1` to write logs to `debug.log`.

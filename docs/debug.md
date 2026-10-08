# Debugging

Set `FRIENDLY_DEBUG` to write a debug log while the app runs.

```bash
FRIENDLY_DEBUG=1 friendly   # startup, requests, errors and panics
FRIENDLY_DEBUG=2 friendly   # also every key and message the app handles
```

The log goes to `$XDG_STATE_HOME/friendly/debug.log`, or `~/.local/state/friendly/debug.log` when `XDG_STATE_HOME`
isn't set. Each run adds to the end of it, and the app prints the path when it quits. Follow it from another terminal
with `tail -f`.

## What it holds

- The build revision, Go version, OS, `TERM`, `TERM_PROGRAM`, `COLORTERM` and whether tmux runs.
- Each config file with its path, whether it exists, and what it set. It also logs whether images use terminal
  graphics and whether the theme picked dark or light.
- Every request with its method, host, path, status and duration, or the error it failed with.
- Every error the app shows, in full.
- A panic while the app handles a message or draws, with its stack. A panic in a background request still prints
  its stack to the terminal as the app quits, but doesn't reach the log.
- At level 2, every key by name and every message by type, with the screen it goes to.

## What it never holds

- Request bodies, headers and query strings, so no tokens, posts or emails.
- Path segments longer than 32 characters, which the log treats as access tokens. They show as `{redacted}`.
- Keys typed as text into the composer, a form or the filter. They show as `typed`, and a paste shows only its length.

The log is safe to attach to a bug report, though it does show the paths of the posts and people you opened.

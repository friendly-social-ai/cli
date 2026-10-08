// Package logging writes the debug log that FRIENDLY_DEBUG turns on. Level 1 logs the startup context, requests,
// errors and panics. Level 2 adds every message the app handles. Request bodies, headers and query strings never reach
// the log, since they hold tokens and personal data.
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// Level reads FRIENDLY_DEBUG. Empty or 0 is off, 1 logs startup, requests, errors and panics, and 2 adds every
// message.
func Level() (int, error) {
	switch value := os.Getenv("FRIENDLY_DEBUG"); value {
	case "", "0":
		return 0, nil
	case "1":
		return 1, nil
	case "2":
		return 2, nil
	default:
		return 0, fmt.Errorf("FRIENDLY_DEBUG: %q is not 0, 1 or 2", value)
	}
}

// Path returns the log file, debug.log in the friendly folder of $XDG_STATE_HOME, or of ~/.local/state when it isn't
// set.
func Path() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("logging: failed to get home directory: %w", err)
		}

		dir = filepath.Join(home, ".local", "state")
	}

	return filepath.Join(dir, "friendly", "debug.log"), nil
}

// Off sends every log nowhere, including the standard log that libraries write to.
func Off() {
	slog.SetDefault(slog.New(slog.DiscardHandler))
}

// Start appends the log at level to the file at path and logs the startup context. It wraps http.DefaultTransport, so
// every request gets logged. The caller closes the file.
func Start(level int, path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("logging: failed to create %s: %w", filepath.Dir(path), err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("logging: failed to open %s: %w", path, err)
	}

	use(f, level)
	http.DefaultTransport = transport{base: http.DefaultTransport}
	slog.Info("start", "revision", revision(), "go", runtime.Version(), "os", runtime.GOOS+"/"+runtime.GOARCH,
		"term", os.Getenv("TERM"), "term_program", os.Getenv("TERM_PROGRAM"), "colorterm", os.Getenv("COLORTERM"),
		"tmux", os.Getenv("TMUX") != "", "level", level)
	return f, nil
}

// use points slog at w, with debug messages at level 2. The standard log goes there too, so library logs show up.
func use(w io.Writer, level int) {
	minLevel := slog.LevelInfo
	if level >= 2 {
		minLevel = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: minLevel})))
	log.SetFlags(0)
}

// revision returns the commit the binary was built from, with +dirty for uncommitted changes, or unknown.
func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	revision, dirty := "unknown", false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if dirty {
		revision += "+dirty"
	}

	return revision
}

// transport logs every request with its status and duration. It logs the host and path only, so the query, headers
// and bodies stay out of the log.
type transport struct {
	base http.RoundTripper
}

// maxSegment is the longest path segment the log keeps. Longer ones count as access tokens, like the one in post
// paths.
const maxSegment = 32

// redact returns path with segments longer than maxSegment replaced.
func redact(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if len(segment) > maxSegment {
			segments[i] = "{redacted}"
		}
	}

	return strings.Join(segments, "/")
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	attrs := []any{"method", req.Method, "url", req.URL.Host + redact(req.URL.Path),
		"duration", time.Since(start).Round(time.Millisecond)}
	if err != nil {
		slog.Error("request", append(attrs, "err", err)...)
		return resp, err
	}

	slog.Info("request", append(attrs, "status", resp.StatusCode)...)
	return resp, nil
}

// Recover logs a panic with its stack and panics again, so the program still restores the terminal. Defer it
// directly, since recover only works there.
func Recover() {
	if r := recover(); r != nil {
		slog.Error("panic", "value", r, "stack", string(debug.Stack()))
		panic(r)
	}
}

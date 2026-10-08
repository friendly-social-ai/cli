package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// capture sends the log at level to a buffer for the test.
func capture(t *testing.T, level int) *bytes.Buffer {
	t.Helper()
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	var buf bytes.Buffer
	use(&buf, level)
	return &buf
}

func TestLevelReadsFriendlyDebug(t *testing.T) {
	for value, want := range map[string]int{"": 0, "0": 0, "1": 1, "2": 2} {
		t.Setenv("FRIENDLY_DEBUG", value)
		if got, err := Level(); err != nil || got != want {
			t.Errorf("Level() with %q = %d, %v, want %d", value, got, err, want)
		}
	}

	t.Setenv("FRIENDLY_DEBUG", "yes")
	if _, err := Level(); err == nil || !strings.Contains(err.Error(), `"yes" is not 0, 1 or 2`) {
		t.Errorf("Level() with yes = %v, want an error", err)
	}
}

func TestPathUsesStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, err := Path(); err != nil || got != filepath.Join("/state", "friendly", "debug.log") {
		t.Errorf("Path() = %q, %v, want /state/friendly/debug.log", got, err)
	}

	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/me")
	if got, err := Path(); err != nil || got != filepath.Join("/home/me", ".local", "state", "friendly", "debug.log") {
		t.Errorf("Path() = %q, %v, want /home/me/.local/state/friendly/debug.log", got, err)
	}
}

func TestRequestLogLeavesOutSecrets(t *testing.T) {
	buf := capture(t, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()

	token := strings.Repeat("t", maxSegment+1)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/community/7/"+token+"?cursor=secret",
		strings.NewReader("body-secret"))
	req.Header.Set("X-Token", "header-secret")
	resp, err := transport{base: http.DefaultTransport}.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() //nolint:errcheck

	out := buf.String()
	for _, want := range []string{"method=POST", "/community/7/{redacted}", "status=418"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q misses %q", out, want)
		}
	}

	for _, secret := range []string{token, "secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("log %q has %q", out, secret)
		}
	}
}

func TestRecoverLogsPanicWithStack(t *testing.T) {
	buf := capture(t, 1)
	defer func() {
		if r := recover(); r != "boom" {
			t.Errorf("recover() = %v, want the panic to go on", r)
		}

		if out := buf.String(); !strings.Contains(out, "msg=panic value=boom") || !strings.Contains(out, "goroutine") {
			t.Errorf("log %q misses the panic and its stack", out)
		}
	}()

	func() {
		defer Recover()
		panic("boom")
	}()
}

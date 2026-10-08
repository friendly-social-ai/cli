package navigation

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTraceLeavesOutTypedText(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	typing := Wrapper{typing: true}
	typing.trace(tea.KeyPressMsg{Code: 'z', Text: "z"})
	typing.trace(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	typing.trace(tea.PasteMsg{Content: "pasted-secret"})
	typing.trace(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	Wrapper{}.trace(tea.KeyPressMsg{Code: 'j', Text: "j"})

	out := buf.String()
	for _, want := range []string{"key=typed typing=true", "key=alt+enter", "msg=paste length=13", "key=j typing=false"} {
		if !strings.Contains(out, want) {
			t.Errorf("trace %q misses %q", out, want)
		}
	}

	for _, secret := range []string{"key=z", "key=space", "pasted-secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("trace %q has %q", out, secret)
		}
	}
}

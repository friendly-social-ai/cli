package config

import (
	"os"
	"path/filepath"
	"testing"
)

// homeWith returns a home folder whose .editorconfig is content, marked as the root so folders above it don't apply.
func homeWith(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".editorconfig"), []byte("root = true\n"+content), 0o600); err != nil {
		t.Fatal(err)
	}

	return home
}

func TestIndentation(t *testing.T) {
	tests := []struct {
		name         string
		setting      int
		editorconfig string
		want         int
	}{
		{"setting wins", 3, "[*]\nindent_size = 2\n", 3},
		{"markdown section", 0, "[*]\nindent_size = 8\n[*.md]\nindent_size = 2\n", 2},
		{"indent_size tab gives tab_width", 0, "[*]\nindent_size = tab\ntab_width = 3\n", 3},
		{"tab style gives tab_width", 0, "[*]\nindent_style = tab\ntab_width = 6\n", 6},
		{"other files only", 0, "[*.go]\nindent_size = 2\n", 4},
		{"out of range", 0, "[*]\nindent_size = 44\n", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Settings{Indent: tt.setting}).Indentation(homeWith(t, tt.editorconfig)); got != tt.want {
				t.Errorf("Indentation() = %d, want %d", got, tt.want)
			}
		})
	}
}

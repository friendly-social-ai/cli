package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// load writes content as a keymap file and loads it. The keys of every action go back to the defaults after the test.
func load(t *testing.T, content string) error {
	t.Helper()
	defaults := make(map[*Action][]string, len(all))
	for _, a := range all {
		defaults[a] = a.keys
	}

	t.Cleanup(func() {
		for a, keys := range defaults {
			a.keys = keys
		}
	})

	path := filepath.Join(t.TempDir(), "keys.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return Load(path)
}

func TestLoadMissingFileKeepsDefaults(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "keys.toml")); err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}

	if got := Community.Reply.Key(); got != "n" {
		t.Errorf("Community.Reply.Key() = %q, want n", got)
	}
}

func TestLoadDefaultsAreValid(t *testing.T) {
	if err := load(t, ""); err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
}

func TestLoadReplacesListedKeys(t *testing.T) {
	err := load(t, `
[community]
reply = "m"

[navigation]
down = ["j", "ctrl+j"]
first = "g"
`)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}

	if Community.Reply.Matches("n") || !Community.Reply.Matches("m") {
		t.Errorf("Community.Reply matches n %v, m %v, want only m", Community.Reply.Matches("n"), Community.Reply.Matches("m"))
	}

	if !Navigation.Down.Matches("ctrl+j") || Navigation.Down.Matches("down") {
		t.Errorf("Navigation.Down = %v, want j and ctrl+j", Navigation.Down.keys)
	}

	if !Navigation.First.Matches("g") {
		t.Errorf("Navigation.First = %v, want g", Navigation.First.keys)
	}

	if got := Community.Edit.Key(); got != "e" {
		t.Errorf("unlisted Community.Edit.Key() = %q, want e", got)
	}
}

func TestLoadUnbindsScreenAction(t *testing.T) {
	if err := load(t, "[community]\nnew_post = []\n"); err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}

	if got := Community.NewPost.Key(); got != "" {
		t.Errorf("Community.NewPost.Key() = %q, want empty", got)
	}

	if Bind("new post", Community.NewPost).Enabled() {
		t.Error("binding of unbound Community.NewPost is enabled")
	}
}

func TestLoadRejectsMistakes(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"unknown action", "[community]\nfly = \"f\"\n", "unknown action community.fly"},
		{"unknown section", "[space]\nfly = \"f\"\n", "unknown action space.fly"},
		{"wrong type", "[community]\nreply = 1\n", "community.reply: want a key or a list of keys"},
		{"unbound navigation", "[navigation]\nquit = \"\"\n", "navigation.quit: needs at least one key"},
		{"unknown key", "[community]\nreply = \"Enter\"\n", `"Enter" is not a key`},
		{"dash modifier", "[navigation]\nhalf_page_down = \"ctrl-d\"\n", `"ctrl-d" is not a key`},
		{"modifier order", "[community]\nreply = \"alt+ctrl+r\"\n", "needs its modifiers in the order"},
		{"shifted character", "[community]\nreply = \"shift+n\"\n", `"shift+n" arrives as "N"`},
		{"sequence", "[community]\nreply = \"z z\"\n", "only navigation.first takes one"},
		{"same scope", "[community]\nreply = \"r\"\n", `community post: "r" is bound to both`},
		{"taken by navigation", "[community]\ncopy = \"j\"\n", `"j" of community.copy is taken by navigation.down`},
		{"back taken by navigation", "[navigation]\nback = \"q\"\n", `"q" of navigation.back is taken by navigation.quit`},
		{"typed text", "[community]\npost = \"s\"\n", `composer: "s" of community.post types text`},
		{"broken toml", "[community\n", "keys.toml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := load(t, tt.content)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load() = %v, want an error with %q", err, tt.want)
			}
		})
	}
}

func TestLoadWithMistakesChangesNothing(t *testing.T) {
	err := load(t, `
[community]
reply = "r"
fly = "f"

[navigation]
quit = ""
`)
	if err == nil {
		t.Fatal("Load() = nil, want an error")
	}

	for _, want := range []string{"unknown action community.fly", "navigation.quit: needs at least one key",
		`community post: "r" is bound to both`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load() = %v, want it to list %q", err, want)
		}
	}

	if got := Community.Reply.Key(); got != "n" {
		t.Errorf("Community.Reply.Key() = %q, want n", got)
	}
}

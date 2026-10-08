package keys

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// Path returns the path of the user keymap, keys.toml in the friendly folder of $XDG_CONFIG_HOME, or of ~/.config
// when it isn't set.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("keys: failed to get home directory: %w", err)
		}

		dir = filepath.Join(home, ".config")
	}

	return filepath.Join(dir, "friendly", "keys.toml"), nil
}

// Load replaces default keys with the ones the user keymap at path lists. Each entry in a section like [community]
// names an action and gives it a key or a list of keys. An empty list unbinds a screen action. A missing file keeps
// the defaults. A file with mistakes changes nothing, and the error lists every mistake.
func Load(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("keys: failed to read %s: %w", path, err)
	}

	var file map[string]map[string]any
	if err := toml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("keys: %s: %w", path, err)
	}

	byID := make(map[string]*Action, len(all))
	for _, a := range all {
		byID[a.ID()] = a
	}

	var problems []string
	chosen := make(map[*Action][]string)
	for _, context := range slices.Sorted(maps.Keys(file)) {
		for _, name := range slices.Sorted(maps.Keys(file[context])) {
			id := context + "." + name
			a, ok := byID[id]
			if !ok {
				problems = append(problems, "unknown action "+id)
				continue
			}

			keys, problem := parse(a, file[context][name])
			if problem != "" {
				problems = append(problems, id+": "+problem)
				continue
			}

			chosen[a] = keys
		}
	}

	keysOf := func(a *Action) []string {
		if keys, ok := chosen[a]; ok {
			return keys
		}

		return a.keys
	}

	problems = append(problems, check(keysOf)...)
	if problems != nil {
		return fmt.Errorf("keys: %s:\n  %s", path, strings.Join(problems, "\n  "))
	}

	for a, keys := range chosen {
		a.keys = keys
	}

	return nil
}

// parse reads the keys the user keymap gives a, a string or a list of strings. It returns what is wrong with them,
// or "".
func parse(a *Action, value any) ([]string, string) {
	var keys []string
	switch value := value.(type) {
	case string:
		if value != "" {
			keys = []string{value}
		}
	case []any:
		for _, item := range value {
			k, ok := item.(string)
			if !ok {
				return nil, "want a key or a list of keys"
			}

			keys = append(keys, k)
		}
	default:
		return nil, "want a key or a list of keys"
	}

	// navigation and common actions move around and leave every screen, so they keep a key
	if len(keys) == 0 && (a.context == "navigation" || a.context == "common") {
		return nil, "needs at least one key"
	}

	for _, k := range keys {
		for _, press := range strings.Split(k, " ") {
			if problem := checkKey(press); problem != "" {
				return nil, problem
			}
		}
	}

	return keys, ""
}

// namedKey matches the keys that have names, as bubbletea writes them.
var namedKey = regexp.MustCompile(`^(enter|tab|backspace|esc|space|up|down|left|right|begin|find|insert|delete|select|` +
	`pgup|pgdown|home|end|f[1-9][0-9]?)$`)

// modifiers are the modifiers a key can have, in the order bubbletea writes them.
var modifiers = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// checkKey returns what is wrong with key k, a single press, or "" when bubbletea reports keys that way. A character
// or a named key comes last, after modifiers in their order, like ctrl+alt+x.
func checkKey(k string) string {
	if utf8.RuneCountInString(k) == 1 {
		return ""
	}

	parts := strings.Split(k, "+")
	base, mods := parts[len(parts)-1], parts[:len(parts)-1]
	if base == "" || utf8.RuneCountInString(base) != 1 && !namedKey.MatchString(base) {
		return fmt.Sprintf("%q is not a key", k)
	}

	last := -1
	for _, mod := range mods {
		i := slices.Index(modifiers, mod)
		if i < 0 {
			return fmt.Sprintf("%q has unknown modifier %q", k, mod)
		}

		if i <= last {
			return fmt.Sprintf("%q needs its modifiers in the order %s", k, strings.Join(modifiers, "+"))
		}

		last = i
	}

	// a shifted character arrives as the character itself, like A for shift+a
	if len(mods) == 1 && mods[0] == "shift" && utf8.RuneCountInString(base) == 1 {
		return fmt.Sprintf("%q arrives as %q", k, strings.ToUpper(base))
	}

	return ""
}

// scope is a set of actions the app offers at the same time, so their keys must differ. A typing scope takes typed
// text, so its keys must not be text or sequences, and navigation doesn't handle its keys.
type scope struct {
	name    string
	typing  bool
	actions []*Action
}

// scopes returns the sets of actions the screens offer together. They copy the actions methods of the screens, so a
// key added there needs adding here.
func scopes() []scope {
	n, c, m := Navigation, Common, Community
	return []scope{
		{"navigation", false, Handled()},
		{"community list", false, []*Action{m.Copy, m.Author, m.NewPost, c.Filter, c.Refresh, c.Cancel}},
		{"community post", false, []*Action{m.Links, m.NextReply, m.PreviousReply, m.Copy, m.Author, c.Refresh,
			m.Reply, m.Edit, m.Delete, n.Back, c.Cancel, m.Root}},
		{"community links", false, []*Action{m.Copy, c.Cancel}},
		{"community menu", false, []*Action{m.Editor, m.Preview, m.Attach, m.PasteImage, m.Discard, c.Cancel, m.Menu}},
		{"community preview", false, []*Action{c.Cancel, m.Preview, m.Post}},
		{"composer", true, []*Action{m.Post, m.Menu, c.Cancel, c.Complete, c.Confirm, c.NextSuggestion,
			c.PreviousSuggestion}},
		{"forms", true, []*Action{c.Confirm, c.NextField, c.PreviousField, c.Cancel}},
		{"activity", false, []*Action{Activity.NextUnread, Activity.ReadAll, c.Refresh}},
		{"people", false, []*Action{People.Connect, People.Skip, c.Filter, c.Refresh, c.Cancel}},
		{"profile", false, []*Action{Profile.Edit, Profile.ToggleEmail, Profile.Logout, c.Cancel}},
		{"user", false, []*Action{User.Connect, User.Remove, User.OpenSocial, n.Back, c.Cancel}},
	}
}

// check returns the problems of the keys keysOf gives every action: a key bound twice in a scope, a key of a screen
// that navigation takes first, a key that never fires because a longer key starts with it, and a key of a typing
// scope that types text or is a sequence.
func check(keysOf func(*Action) []string) []string {
	nav := Handled()
	var problems []string
	for _, s := range scopes() {
		// navigation handles its keys on every screen that isn't typing, so they count there too. Problems among
		// navigation keys show only in the navigation scope.
		actions := s.actions
		if !s.typing && s.name != "navigation" {
			actions = append(slices.Clone(nav), actions...)
		}

		inNav := func(a *Action) bool {
			return s.name != "navigation" && slices.Contains(nav, a)
		}

		owners := make(map[string]*Action)
		for _, a := range actions {
			for _, k := range keysOf(a) {
				switch {
				case s.typing && strings.Contains(k, " "):
					problems = append(problems, fmt.Sprintf("%s: %q of %s is a sequence, which doesn't work while typing",
						s.name, k, a.ID()))
				case s.typing && (utf8.RuneCountInString(k) == 1 || k == "space"):
					problems = append(problems,
						fmt.Sprintf("%s: %q of %s types text, pick a key with ctrl or alt", s.name, k, a.ID()))
				}

				other, ok := owners[k]
				switch {
				case !ok || other == a || inNav(other) && inNav(a):
				case inNav(other):
					problems = append(problems, fmt.Sprintf("%s: %q of %s is taken by %s", s.name, k, a.ID(), other.ID()))
				default:
					problems = append(problems,
						fmt.Sprintf("%s: %q is bound to both %s and %s", s.name, k, other.ID(), a.ID()))
				}

				if !ok {
					owners[k] = a
				}
			}
		}

		// there is no timeout, so a key that starts a longer one always waits for more
		for _, k := range slices.Sorted(maps.Keys(owners)) {
			presses := strings.Split(k, " ")
			for i := 1; i < len(presses); i++ {
				prefix := strings.Join(presses[:i], " ")
				if short, ok := owners[prefix]; ok && (!inNav(short) || !inNav(owners[k])) {
					problems = append(problems, fmt.Sprintf("%s: %q of %s never fires, %q of %s starts with it",
						s.name, prefix, short.ID(), k, owners[k].ID()))
				}
			}
		}
	}

	return problems
}

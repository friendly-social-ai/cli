package ui

//go:generate go run gen_shortcodes.go

import (
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark-emoji/definition"
)

// shortcodePattern matches an emoji shortcode like :smile:, with the characters markdown accepts in its name.
var shortcodePattern = regexp.MustCompile(`:([A-Za-z0-9_+-]+):`)

// Shortcode is an emoji and its name, like smile for 😄.
type Shortcode struct {
	Name  string
	Emoji string
}

// lookup returns the emoji of shortcode name. Custom GitHub emoji like octocat have no unicode, so they don't count.
func lookup(name string) (string, bool) {
	e, ok := definition.Github().Get(name)
	if !ok || !e.IsUnicode() {
		return "", false
	}

	return string(e.Unicode), true
}

// Emojize turns emoji shortcodes in text into emoji, for text shown without markdown. Unknown ones stay as typed.
func Emojize(text string) string {
	return shortcodePattern.ReplaceAllStringFunc(text, func(match string) string {
		if e, ok := lookup(match[1 : len(match)-1]); ok {
			return e
		}

		return match
	})
}

// SuggestShortcodes returns up to limit shortcodes whose names contain query. The ones starting with it go first, and
// shorter ones first among them, so :sm suggests smile before small_blue_diamond.
func SuggestShortcodes(query string, limit int) []Shortcode {
	query = strings.ToLower(query)
	var starts, contains []Shortcode
	for _, name := range shortcodes {
		if !strings.Contains(name, query) {
			continue
		}

		e, ok := lookup(name)
		if !ok {
			continue
		}

		if strings.HasPrefix(name, query) {
			starts = append(starts, Shortcode{Name: name, Emoji: e})
		} else {
			contains = append(contains, Shortcode{Name: name, Emoji: e})
		}
	}

	// the stable sort keeps names of the same length in alphabetical order
	slices.SortStableFunc(starts, func(a, b Shortcode) int {
		return len(a.Name) - len(b.Name)
	})

	all := append(starts, contains...)
	return all[:min(len(all), limit)]
}

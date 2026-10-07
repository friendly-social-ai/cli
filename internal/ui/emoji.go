package ui

import (
	"regexp"

	"github.com/yuin/goldmark-emoji/definition"
)

// shortcodePattern matches an emoji shortcode like :smile:, with the characters markdown accepts in its name.
var shortcodePattern = regexp.MustCompile(`:([A-Za-z0-9_+-]+):`)

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

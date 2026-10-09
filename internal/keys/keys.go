// Package keys names every action the user triggers with keys, with its default keys. Screens and navigation bind
// keys through these actions, so hints and text that name a key show the key that is bound.
package keys

import (
	"slices"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
)

// Action is something the user does with keys. A key with spaces is a sequence of presses, like "g g".
type Action struct {
	context, name string
	keys          []string
}

// all holds every action in the order they are defined, for Load to find them by ID.
var all []*Action

func define(context, name string, keys ...string) *Action {
	a := &Action{context: context, name: name, keys: keys}
	all = append(all, a)
	return a
}

// ID returns the context and name of a, like community.reply.
func (a *Action) ID() string {
	return a.context + "." + a.name
}

// Matches reports whether k is one of the keys of a. A sequence matches only as a whole.
func (a *Action) Matches(k string) bool {
	return slices.Contains(a.keys, k)
}

// Key returns the first key of a for text that names it, or "" when the user keymap unbinds a.
func (a *Action) Key() string {
	if len(a.keys) == 0 {
		return ""
	}

	return display(a.keys[0])
}

// display renders key k for hints. A sequence of single characters shows joined, like gg.
func display(k string) string {
	parts := strings.Split(k, " ")
	for _, part := range parts {
		if utf8.RuneCountInString(part) != 1 {
			return k
		}
	}

	return strings.Join(parts, "")
}

// Bind returns a binding of the keys of actions in order, with help desc. Hints show the first key.
func Bind(desc string, actions ...*Action) key.Binding {
	var bound []string
	for _, a := range actions {
		bound = append(bound, a.keys...)
	}

	// an action the user keymap unbinds has no keys, and a disabled binding neither matches nor shows
	if len(bound) == 0 {
		return key.NewBinding(key.WithDisabled())
	}

	return key.NewBinding(key.WithKeys(bound...), key.WithHelp(display(bound[0]), desc))
}

// Hint returns a binding that only describes keys handled elsewhere, shown as the first label with the others as
// alternatives.
func Hint(desc string, labels ...string) key.Binding {
	return key.NewBinding(key.WithKeys(labels...), key.WithHelp(labels[0], desc))
}

// Pair returns labels for a and b side by side, one per key both have, like "j / k" and "down / up". The second key
// drops modifiers it shares with the first, as in "ctrl+d / u".
func Pair(a, b *Action, sep string) []string {
	var labels []string
	for i := range min(len(a.keys), len(b.keys)) {
		first, second := display(a.keys[i]), display(b.keys[i])
		if plus := strings.LastIndex(first, "+"); plus >= 0 && strings.HasPrefix(second, first[:plus+1]) {
			second = second[plus+1:]
		}

		labels = append(labels, first+sep+second)
	}

	return labels
}

// Label returns the first label of Pair.
func Label(a, b *Action, sep string) string {
	return Pair(a, b, sep)[0]
}

// Handled returns the actions navigation handles before screens see a key. Navigation.Back is not one of them, since
// screens decide where back goes.
func Handled() []*Action {
	n := Navigation
	return append([]*Action{n.Quit, n.Help, n.Open, n.Down, n.Up, n.First, n.Last, n.HalfPageDown, n.HalfPageUp},
		n.Tabs...)
}

// Navigation holds keys that navigation handles on every screen.
var Navigation = struct {
	Quit, Help, Open, Back, Down, Up, First, Last, HalfPageDown, HalfPageUp *Action
	// Tabs switch to the router's tabs, in order
	Tabs []*Action
}{
	Quit:         define("navigation", "quit", "q"),
	Help:         define("navigation", "help", "?"),
	Open:         define("navigation", "open", "l", "enter", "right"),
	Back:         define("navigation", "back", "h", "left"),
	Down:         define("navigation", "down", "j", "down"),
	Up:           define("navigation", "up", "k", "up"),
	First:        define("navigation", "first", "g g"),
	Last:         define("navigation", "last", "G"),
	HalfPageDown: define("navigation", "half_page_down", "ctrl+d"),
	HalfPageUp:   define("navigation", "half_page_up", "ctrl+u"),
	Tabs: []*Action{
		define("navigation", "tab_1", "1"),
		define("navigation", "tab_2", "2"),
		define("navigation", "tab_3", "3"),
		define("navigation", "tab_4", "4"),
	},
}

// Common holds keys that several screens share.
var Common = struct {
	Cancel, Confirm, NextField, PreviousField, Complete, NextSuggestion, PreviousSuggestion, Refresh, Filter *Action
}{
	Cancel:             define("common", "cancel", "esc"),
	Confirm:            define("common", "confirm", "enter"),
	NextField:          define("common", "next_field", "tab", "down"),
	PreviousField:      define("common", "previous_field", "shift+tab", "up"),
	Complete:           define("common", "complete", "tab"),
	NextSuggestion:     define("common", "next_suggestion", "down", "ctrl+n"),
	PreviousSuggestion: define("common", "previous_suggestion", "up", "ctrl+p"),
	Refresh:            define("common", "refresh", "R"),
	Filter:             define("common", "filter", "/"),
}

// Community holds keys of the community screen, with its composer and its menu.
var Community = struct {
	NewPost, Reply, Links, Copy, Author, Edit, Delete, NextReply, PreviousReply, Root *Action
	Menu, Post, Editor, Preview, Attach, PasteImage, Discard, Indent, Outdent         *Action
}{
	NewPost:       define("community", "new_post", "n"),
	Reply:         define("community", "reply", "r"),
	Links:         define("community", "links", "o"),
	Copy:          define("community", "copy", "y"),
	Author:        define("community", "author", "@"),
	Edit:          define("community", "edit", "e"),
	Delete:        define("community", "delete", "d"),
	NextReply:     define("community", "next_reply", "J"),
	PreviousReply: define("community", "previous_reply", "K"),
	Root:          define("community", "root", "H"),
	Menu:          define("community", "menu", "ctrl+o"),
	Post:          define("community", "post", "alt+enter"),
	Editor:        define("community", "editor", "e"),
	Preview:       define("community", "preview", "p"),
	Attach:        define("community", "attach", "a"),
	PasteImage:    define("community", "paste_image", "v"),
	Discard:       define("community", "discard", "x"),
	Indent:        define("community", "indent", "tab"),
	Outdent:       define("community", "outdent", "shift+tab"),
}

// Activity holds keys of the activity screen.
var Activity = struct{ NextUnread, ReadAll *Action }{
	NextUnread: define("activity", "next_unread", "n"),
	ReadAll:    define("activity", "read_all", "m"),
}

// People holds keys of the people screen.
var People = struct{ Connect, Skip *Action }{
	Connect: define("people", "connect", "a"),
	Skip:    define("people", "skip", "x"),
}

// Profile holds keys of the profile screen.
var Profile = struct{ Edit, ToggleEmail, Logout *Action }{
	Edit:        define("profile", "edit", "e"),
	ToggleEmail: define("profile", "toggle_email", "v"),
	Logout:      define("profile", "logout", "x"),
}

// User holds keys of the screen with the profile of another user.
var User = struct{ Connect, Remove, OpenSocial *Action }{
	Connect:    define("user", "connect", "a"),
	Remove:     define("user", "remove", "x"),
	OpenSocial: define("user", "open_social", "o"),
}

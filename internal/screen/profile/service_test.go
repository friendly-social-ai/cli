package profile

import (
	"errors"
	"testing"
)

func TestParseInvite(t *testing.T) {
	const token = "Ab3_-.~xYz012345"
	tests := []struct {
		name, invite string
	}{
		{"bare token", token},
		{"invite link", "https://invite.getfriend.ly/" + token},
		{"trailing slash and spaces", "  https://invite.getfriend.ly/" + token + "/\n"},
		{"older reference link", "https://getfriend.ly/#?reference=add/" + token},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInvite(tt.invite)
			if err != nil {
				t.Fatal(err)
			}
			if got.Value() != token {
				t.Errorf("parseInvite(%q) = %q, want %q", tt.invite, got.Value(), token)
			}
		})
	}

	for _, invite := range []string{"", "https://invite.getfriend.ly/", "https://invite.getfriend.ly/short"} {
		if _, err := parseInvite(invite); !errors.Is(err, errInvalidInvite) {
			t.Errorf("parseInvite(%q) error = %v, want errInvalidInvite", invite, err)
		}
	}
}

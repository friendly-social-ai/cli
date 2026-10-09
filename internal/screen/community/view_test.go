package community

import (
	"testing"

	sdk "github.com/friendly-social-ai/golang-sdk"
)

func TestFirstLineShowsLeadingDisplayMath(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"dollars", "$$\n\\alpha^2\n$$\nmore", "α²"},
		{"math code block", "```math\n\\alpha^2\n```", "α²"},
		{"unsupported shows tex", "$$\na \\\\\nb\n$$", `a \\ b`},
		{"text before math", "see\n$$\n\\alpha\n$$", "see"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := sdk.NewCommunityPostText(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			if got := FirstLine(sdk.CommunityPost{Text: &text}); got != tt.want {
				t.Errorf("FirstLine(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

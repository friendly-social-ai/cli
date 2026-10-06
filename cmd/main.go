package main

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/friendly-social/cli/internal/navigation"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/activity"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/screen/community"
	"github.com/friendly-social/cli/internal/screen/home"
	"github.com/friendly-social/cli/internal/screen/people"
	"github.com/friendly-social/cli/internal/screen/profile"
	"github.com/friendly-social/cli/internal/screen/register"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

func main() {
	// standard log goes to debug.log only when DEBUG is set, since the program draws over stdout
	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close() //nolint:errcheck
	} else {
		log.SetOutput(io.Discard)
	}

	ui.SetTheme(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))
	graphics := ui.NewGraphics()

	client := sdk.NewClient()
	screens := []screen.Model{
		home.New(),
		community.New(community.NewService(client), graphics),
		activity.New(activity.NewService(client)),
		people.New(people.NewService(client)),
		profile.New(profile.NewService(client)),
		register.New(register.NewService(client)),
		auth.New(auth.NewService(client)),
	}

	router := router.NewRouter(screens)
	wrapper := navigation.NewVimWrapper(router)

	p := tea.NewProgram(wrapper, options()...)
	_, err := p.Run()
	if graphics != nil {
		graphics.Close(os.Stdout)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to run app router:", err)
		os.Exit(1)
	}
}

// options returns program options. Inside tmux, color detection ignores COLORTERM and asks `tmux info`, which
// describes the inner terminfo rather than the client. It then picks 256 colors, which breaks image placeholders
// because they carry the image ID in their 24-bit color. So options trusts COLORTERM instead.
func options() []tea.ProgramOption {
	if c := os.Getenv("COLORTERM"); c == "truecolor" || c == "24bit" {
		return []tea.ProgramOption{tea.WithColorProfile(colorprofile.TrueColor)}
	}

	return nil
}

package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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
	}

	out := ui.NewOutput(os.Stdout)
	graphics := ui.NewGraphics(out)

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

	p := tea.NewProgram(wrapper, tea.WithAltScreen(), tea.WithOutput(out))
	_, err := p.Run()
	if graphics != nil {
		graphics.Close()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to run app router:", err)
		os.Exit(1)
	}
}

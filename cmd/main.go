package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/friendly-social/cli/internal/navigation"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/screen/home"
	"github.com/friendly-social/cli/internal/screen/people"
	"github.com/friendly-social/cli/internal/screen/profile"
	"github.com/friendly-social/cli/internal/screen/register"
	sdk "github.com/friendly-social/golang-sdk"
)

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	client := sdk.NewClient()
	screens := []screen.Model{
		home.New(),
		people.New(people.NewService(client)),
		profile.New(profile.NewService(client)),
		register.New(register.NewService(client)),
		auth.New(auth.NewService(client)),
	}

	router := router.NewRouter(screens)
	wrapper := navigation.NewVimWrapper(router)

	p := tea.NewProgram(wrapper, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to run app router:", err)
		os.Exit(1)
	}
}

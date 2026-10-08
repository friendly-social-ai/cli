package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/navigation"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/activity"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/screen/community"
	"github.com/friendly-social-ai/cli/internal/screen/home"
	"github.com/friendly-social-ai/cli/internal/screen/people"
	"github.com/friendly-social-ai/cli/internal/screen/profile"
	"github.com/friendly-social-ai/cli/internal/screen/register"
	"github.com/friendly-social-ai/cli/internal/screen/user"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
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

	// a keymap with mistakes stops the app before it draws, so the user sees every mistake at once
	path, err := keys.Path()
	if err == nil {
		err = keys.Load(path)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ui.SetTheme(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))
	graphics := ui.NewGraphics()

	// the transport reports a rejected session to the program, which is assigned below before any request runs
	var p *tea.Program
	client := sdk.NewClient().WithHTTPClient(&http.Client{
		Timeout: 30 * time.Second,
		Transport: auth.Transport{Expired: func() {
			p.Send(router.TargetMsg{Type: screen.TypeAuth, Inner: auth.ExpiredMsg{}})
		}},
	})
	screens := []screen.Model{
		home.New(),
		community.New(community.NewService(client), graphics),
		activity.New(activity.NewService(client)),
		people.New(people.NewService(client)),
		profile.New(profile.NewService(client)),
		register.New(register.NewService(client)),
		auth.New(auth.NewService(client)),
		user.New(user.NewService(client)),
	}

	wrapper := navigation.NewWrapper(router.NewRouter(screens))

	p = tea.NewProgram(wrapper, options()...)
	// tmux draws emoji at their grapheme width, which is how lipgloss measures them. It doesn't answer the query for
	// mode 2027, so the renderer would measure with wcwidth and draw updates one cell off after an emoji with a
	// variation selector or a skin tone. This message switches the renderer to grapheme width.
	if os.Getenv("TMUX") != "" {
		go p.Send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
	}

	_, err = p.Run()
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

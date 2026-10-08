package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social-ai/cli/internal/config"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/logging"
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
	os.Exit(run())
}

// run starts the app and returns its exit code. With FRIENDLY_DEBUG set, it prints the path of the debug log when it
// returns, so a bug report can attach the log.
func run() int {
	level, err := logging.Level()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// the program draws over stdout, so logs go to a file and only while debugging
	logging.Off()
	if level > 0 {
		path, err := logging.Path()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		f, err := logging.Start(level, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		defer f.Close() //nolint:errcheck
		defer fmt.Fprintln(os.Stderr, "debug log:", path)
	}

	// config files with mistakes stop the app before it draws, so the user sees every mistake at once
	settings, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		slog.Error("config", "err", err)
		return 1
	}

	graphics := newGraphics(settings.Images)
	slog.Info("images", "setting", settings.Images, "graphics", graphics != nil)

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
		community.New(community.NewService(client), graphics, settings.Images == "off"),
		activity.New(activity.NewService(client)),
		people.New(people.NewService(client)),
		profile.New(profile.NewService(client)),
		register.New(register.NewService(client)),
		auth.New(auth.NewService(client)),
		user.New(user.NewService(client)),
	}

	wrapper := navigation.NewWrapper(router.NewRouter(screens, settings.Refresh))

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
		slog.Error("run", "err", err)
		return 1
	}

	slog.Info("quit")
	return 0
}

// loadConfig loads the user settings, keymap and theme, and reports the mistakes of all three. The theme asks the
// terminal for its background only in auto mode.
func loadConfig() (config.Settings, error) {
	var paths [3]string
	for i, name := range []string{"config.toml", "keys.toml", "theme.toml"} {
		path, err := config.Path(name)
		if err != nil {
			return config.Defaults, err
		}

		paths[i] = path
	}

	settings, err := config.Load(paths[0])
	return settings, errors.Join(err, keys.Load(paths[1]), ui.LoadTheme(paths[2], func() bool {
		return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	}))
}

// newGraphics returns the terminal graphics the images setting asks for, or nil to draw images with half-blocks or
// not at all.
func newGraphics(images string) *ui.Graphics {
	switch images {
	case "auto":
		return ui.NewGraphics()
	case "graphics":
		return ui.ForceGraphics()
	}

	return nil
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

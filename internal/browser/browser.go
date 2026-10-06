// Package browser opens links and files with the default applications of the system.
package browser

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

// Open opens http or https link in the default browser without waiting for it. Other schemes are refused, so that
// content from posts can't launch local files or applications.
func Open(link string) error {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("not a web link: %s", link)
	}

	return start(u.String())
}

// OpenFile opens local file with its default application. Only for files created by the program itself.
func OpenFile(path string) error {
	return start(path)
}

// start runs the system opener for target without waiting for it. Its output is discarded, it would draw over
// the program otherwise.
func start(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open %s: %w", target, err)
	}

	go cmd.Wait() //nolint:errcheck

	return nil
}

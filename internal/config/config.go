// Package config finds the files the user configures the app with.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Path returns the path of the config file name, in the friendly folder of $XDG_CONFIG_HOME, or of ~/.config when it
// isn't set.
func Path(name string) (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: failed to get home directory: %w", err)
		}

		dir = filepath.Join(home, ".config")
	}

	return filepath.Join(dir, "friendly", name), nil
}

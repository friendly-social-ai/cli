// Package config finds the files the user configures the app with.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Path returns the path of the config file name. It is in $FRIENDLY_CONFIG_DIR when that is set, and otherwise in the
// friendly folder of $XDG_CONFIG_HOME, or of ~/.config. A FRIENDLY_CONFIG_DIR that isn't a folder is an error, since
// the app would otherwise run with the defaults and hide the typo.
func Path(name string) (string, error) {
	if dir := os.Getenv("FRIENDLY_CONFIG_DIR"); dir != "" {
		info, err := os.Stat(dir)
		if err != nil {
			return "", fmt.Errorf("FRIENDLY_CONFIG_DIR: %w", err)
		}

		if !info.IsDir() {
			return "", fmt.Errorf("FRIENDLY_CONFIG_DIR: %s is not a folder", dir)
		}

		return filepath.Join(dir, name), nil
	}

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

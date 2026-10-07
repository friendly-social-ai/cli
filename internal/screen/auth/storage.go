package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/friendly-social-ai/golang-sdk"
)

const (
	saveFile   = "user.json"
	saveFolder = "friendly"
)

// Load reads saved authorization from user cache directory. Returns nil if nothing is saved.
func Load() (*sdk.Authorization, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("auth: failed to get user cache dir: %w", err)
	}

	saveFile := filepath.Join(cacheDir, saveFolder, saveFile)
	_, err = os.Stat(saveFile)
	if os.IsNotExist(err) {
		return nil, nil
	}

	userBytes, err := os.ReadFile(saveFile)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to read user bytes: %w", err)
	}

	user := new(sdk.Authorization)
	err = json.Unmarshal(userBytes, user)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to unmarshal user bytes: %w", err)
	}

	return user, nil
}

// Save writes authorization to user cache directory so it can be restored with Load.
func Save(user *sdk.Authorization) error {
	cacheFolder, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("auth: failed to get user cache directory: %w", err)
	}

	saveFolder := filepath.Join(cacheFolder, saveFolder)
	err = os.MkdirAll(saveFolder, 0700)
	if err != nil {
		return fmt.Errorf("auth: failed to create save folder: %w", err)
	}

	userBytes, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("auth: failed to marshal user data: %w", err)
	}

	saveFile := filepath.Join(saveFolder, saveFile)
	err = os.WriteFile(saveFile, userBytes, 0600)
	if err != nil {
		return fmt.Errorf("auth: failed to write user data to save file: %w", err)
	}

	return nil
}

// Clear removes saved authorization from user cache directory.
func Clear() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("auth: failed to get user cache dir: %w", err)
	}

	err = os.Remove(filepath.Join(cacheDir, saveFolder, saveFile))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("auth: failed to remove save file: %w", err)
	}

	return nil
}

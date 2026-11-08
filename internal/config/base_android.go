//go:build android

package config

import (
	"errors"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// baseDir returns the app's private internal storage. Android app processes
// do not set $HOME, so os.UserConfigDir fails there; getFilesDir() is the
// directory Android expects databases and persistent files to live in.
func baseDir() (string, error) {
	if dir := application.Android.StoragePath(); dir != "" {
		return dir, nil
	}
	// Some OEM builds report an empty files dir; $HOME is a usable last resort
	// when the platform does provide one.
	if dir := os.Getenv("HOME"); dir != "" {
		return dir, nil
	}
	return "", errors.New("android: app files directory is unavailable")
}

//go:build !android

package config

import "os"

// baseDir returns the OS per-user configuration root.
func baseDir() (string, error) { return os.UserConfigDir() }

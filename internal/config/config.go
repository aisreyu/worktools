// Package config resolves per-user application paths and stores non-secret
// settings. Secrets never live here.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// AppDir returns the per-user directory that holds the database and settings.
func AppDir() (string, error) {
	base, err := baseDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "notebook")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// DBPath returns the SQLite database path.
func DBPath() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "notebook.db"), nil
}

// Platform reports the runtime OS (used by the UI to adjust shortcuts).
func Platform() string { return runtime.GOOS }

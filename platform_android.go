//go:build android

package main

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// preparePlatform points TMPDIR at the app's private storage. Android app
// processes do not define TMPDIR, so os.TempDir() would resolve to /tmp, which
// does not exist there, and every os.MkdirTemp/CreateTemp call would fail.
func preparePlatform() {
	dir := application.Android.StoragePath()
	if dir == "" {
		return
	}
	tmp := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmp, 0o700); err == nil {
		_ = os.Setenv("TMPDIR", tmp)
	}
}

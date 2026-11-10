//go:build darwin

package screenshot

import "os/exec"

// copyImageFile loads a PNG file into the macOS general pasteboard as an image
// using AppleScript (no cgo required).
func copyImageFile(path string) error {
	script := `set the clipboard to (read (POSIX file "` + path + `") as «class PNGf»)`
	return exec.Command("osascript", "-e", script).Run()
}

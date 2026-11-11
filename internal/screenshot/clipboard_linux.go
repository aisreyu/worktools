//go:build linux

package screenshot

import (
	"os"
	"os/exec"
)

// copyImageFile loads a PNG file into the clipboard. Wayland (wl-copy) is
// preferred, falling back to X11 (xclip).
func copyImageFile(path string) error {
	if _, err := exec.LookPath("wl-copy"); err == nil {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		cmd := exec.Command("wl-copy", "--type", "image/png")
		cmd.Stdin = f
		return cmd.Run()
	}
	return exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-i", path).Run()
}

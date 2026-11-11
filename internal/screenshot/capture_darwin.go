//go:build darwin

package screenshot

import (
	"fmt"
	"os/exec"
)

// captureRectRaw grabs a rectangle. On macOS screencapture's -R takes points
// (DIP), so the scale factor is intentionally ignored: the OS captures the
// full native resolution for that rectangle automatically.
func captureRectRaw(r Rect, _ float64) ([]byte, error) {
	if r.Width <= 0 || r.Height <= 0 {
		return nil, fmt.Errorf("invalid capture rect %dx%d", r.Width, r.Height)
	}
	path := tmpCapturePath(".png")
	spec := fmt.Sprintf("-R%d,%d,%d,%d", r.X, r.Y, r.Width, r.Height)
	// -x suppresses the shutter sound; -t png selects the format.
	out, err := exec.Command("screencapture", "-x", "-t", "png", spec, path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("screencapture failed: %s: %w", string(out), err)
	}
	return readAndRemove(path)
}

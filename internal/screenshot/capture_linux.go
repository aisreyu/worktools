//go:build linux

package screenshot

import (
	"fmt"
	"os/exec"
)

// captureRectRaw grabs a rectangle on Linux. It prefers `grim` (Wayland) and
// falls back to ImageMagick's `import` (X11). Both accept the geometry in
// physical pixels, so the DIP rect is scaled by the display scale factor.
func captureRectRaw(r Rect, scale float64) ([]byte, error) {
	if r.Width <= 0 || r.Height <= 0 {
		return nil, fmt.Errorf("invalid capture rect %dx%d", r.Width, r.Height)
	}
	if scale <= 0 {
		scale = 1
	}
	x := int(float64(r.X) * scale)
	y := int(float64(r.Y) * scale)
	w := int(float64(r.Width) * scale)
	h := int(float64(r.Height) * scale)

	geom := fmt.Sprintf("%dx%d+%d+%d", w, h, x, y)
	path := tmpCapturePath(".png")

	if grim, err := exec.LookPath("grim"); err == nil {
		out, err := exec.Command(grim, "-g", geom, path).CombinedOutput()
		if err == nil {
			return readAndRemove(path)
		}
		_ = removePath(path)
		// fall through to ImageMagick
	}
	if imp, err := exec.LookPath("import"); err == nil {
		out, err := exec.Command(imp, "-window", "root", "-crop", geom, path).CombinedOutput()
		if err != nil {
			_ = removePath(path)
			return nil, fmt.Errorf("import capture failed: %s: %w", string(out), err)
		}
		return readAndRemove(path)
	}
	return nil, fmt.Errorf("no screen-capture tool found: install grim (Wayland) or ImageMagick (X11)")
}

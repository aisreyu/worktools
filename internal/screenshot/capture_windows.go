//go:build windows

package screenshot

import (
	"fmt"
	"os/exec"
)

// captureRectRaw grabs a rectangle using a short PowerShell script built on
// System.Drawing. The DIP rect is scaled up by the display scale factor so the
// captured bitmap keeps the native resolution.
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

	path := tmpCapturePath(".png")
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
$bmp = New-Object System.Drawing.Bitmap %d, %d
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen(%d, %d, 0, 0, $bmp.Size)
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose(); $bmp.Dispose()
`, w, h, x, y, path)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("powershell capture failed: %s: %w", string(out), err)
	}
	return readAndRemove(path)
}

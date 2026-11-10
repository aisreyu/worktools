package screenshot

import "os"

// CopyImageToClipboard places a PNG image (raw bytes) onto the system
// clipboard. Each platform implements copyImageFile.
func CopyImageToClipboard(png []byte) error {
	path := tmpCapturePath(".png")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		return err
	}
	defer removePath(path)
	return copyImageFile(path)
}

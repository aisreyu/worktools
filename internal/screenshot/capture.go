package screenshot

import (
	"fmt"
	"os"
	"path/filepath"
)

// tmpCapturePath returns a fresh temp file path for a capture tool to write to.
func tmpCapturePath(ext string) string {
	f, err := os.CreateTemp("", "wt-scr-*"+ext)
	if err != nil {
		// Fall back to a deterministic path; readAndRemove tolerates missing.
		return filepath.Join(os.TempDir(), fmt.Sprintf("wt-scr-%d%s", os.Getpid(), ext))
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name) // let the capture tool create it anew
	return name
}

// readAndRemove reads path and deletes it, returning the bytes.
func readAndRemove(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	_ = os.Remove(path)
	if err != nil {
		return nil, fmt.Errorf("read capture: %w", err)
	}
	return b, nil
}

// removePath best-effort deletes a temp file, ignoring errors.
func removePath(path string) error { return os.Remove(path) }

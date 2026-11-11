//go:build !darwin && !windows && !linux

package screenshot

import "fmt"

// captureRectRaw is unsupported on platforms without a known capture backend.
func captureRectRaw(r Rect, scale float64) ([]byte, error) {
	return nil, fmt.Errorf("screen capture is not supported on this platform")
}

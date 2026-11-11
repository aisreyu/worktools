package screenshot

// CaptureRegion captures the given rectangle (DIP, global screen space) and
// returns the shot together with its base64 PNG. scale is the display scale
// factor (1 on non-Retina, 2 on Retina); it is only used by the pixel-based
// Windows/Linux backends.
func CaptureRegion(r Rect, scale float64) (Shot, string, error) {
	return captureB64(r, scale)
}

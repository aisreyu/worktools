// Package screenshot implements the screen-capture feature: full-screen and
// region capture, scrolled capture with automatic stitching, and OCR of a
// selected region. It is a pure-Go, OS-agnostic facade that shells out to the
// platform capture tool (screencapture / PowerShell / grim) and to the local
// tesseract binary for text recognition.
//
// Coordinates: every rect handed to the capture backends is expressed in DIP
// (device-independent pixels / points) in the *global screen coordinate space*,
// whose origin is the primary display's top-left corner. This is exactly the
// space Wails reports via application.Screen.Bounds, so the UI can compute a
// selection rect with simple addition and hand it straight to the backend. The
// captured PNG keeps the display's native resolution (2x on Retina), which is
// ideal for both saving and OCR.
package screenshot

// Rect is a rectangle in DIP relative to the primary display's top-left corner.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// primaryScreenOrigin returns the top-left corner of the primary display in the
// capture tools' coordinate space. The primary display is the global origin by
// definition, so this is always (0, 0).
func primaryScreenOrigin() (int, int) { return 0, 0 }

//go:build !darwin || !cgo

package clipboard

// readOSClipboard is a no-op on platforms without the native pasteboard reader.
// The app targets macOS, so other platforms fall back to whatever the caller
// supplies (for example, the Wails clipboard text API).
func readOSClipboard() (Capture, int64) { return Capture{}, 0 }

package services

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/aisreyu/worktools/internal/config"
	"github.com/aisreyu/worktools/internal/screenshot"
)

// Event names emitted by the screenshot service. The main window subscribes to
// these to pick up results produced inside the full-screen overlay windows.
const (
	EventShotResult  = "wt:shot:result"
	EventShotCancel  = "wt:shot:cancel"
	EventShotError   = "wt:shot:error"
	eventOverlayOpen = "wt:overlay:open"
)

// ScreenshotService exposes screen capture, scrolling stitch and OCR to the UI.
type ScreenshotService struct {
	mu        sync.Mutex
	app       *application.App
	overlays  []*application.WebviewWindow
	screens   []ScreenInfo
	lastRect  screenshot.Rect
	lastScale float64
}

// NewScreenshotService creates the service. The app handle is attached later
// (after application.New) via AttachApp.
func NewScreenshotService() *ScreenshotService { return &ScreenshotService{} }

// AttachApp wires the running application in so the service can create the
// full-screen overlay windows and read screen geometry.
func (s *ScreenshotService) AttachApp(app *application.App) { s.app = app }

// ShotResult is a captured image returned to the UI.
type ShotResult struct {
	Data   string  `json:"data"`   // base64 PNG (no data-URI prefix)
	Width  int     `json:"width"`  // pixel width
	Height int     `json:"height"` // pixel height
	Scale  float64 `json:"scale"`  // display scale factor used
	Path   string  `json:"path"`   // non-empty when saved straight to disk
}

// ScreenInfo describes one display for the overlay windows.
type ScreenInfo struct {
	Index   int     `json:"index"`
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Scale   float64 `json:"scale"`
	Primary bool    `json:"primary"`
}

// ScreenshotInfo reports platform/tool availability for the UI.
type ScreenshotInfo struct {
	Platform  string         `json:"platform"`
	OCR       map[string]any `json:"ocr"`
	Supported bool           `json:"supported"`
}

// Info returns capability information for the screenshot view.
func (s *ScreenshotService) Info() ScreenshotInfo {
	p := config.Platform()
	return ScreenshotInfo{
		Platform:  p,
		OCR:       screenshot.OCREngineInfo(),
		Supported: p == "darwin" || p == "windows" || p == "linux",
	}
}

// Screens returns the displays available for capture. The overlay windows are
// positioned from this list.
func (s *ScreenshotService) Screens() []ScreenInfo {
	out := []ScreenInfo{}
	if s.app == nil || s.app.Screen == nil {
		return out
	}
	for i, sc := range s.app.Screen.GetAll() {
		out = append(out, ScreenInfo{
			Index:   i,
			ID:      sc.ID,
			Name:    sc.Name,
			X:       sc.Bounds.X,
			Y:       sc.Bounds.Y,
			Width:   sc.Bounds.Width,
			Height:  sc.Bounds.Height,
			Scale:   float64(sc.ScaleFactor),
			Primary: sc.IsPrimary,
		})
	}
	return out
}

// BeginRegion opens a transparent, always-on-top overlay window on every
// display so the user can drag out a selection. The overlay renders at the URL
// /#/overlay?idx=N; when the user commits a selection the overlay calls
// CommitRegion, and the result is emitted as EventShotResult on the app bus.
func (s *ScreenshotService) BeginRegion() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return fmt.Errorf("screenshot service is not ready")
	}
	s.closeOverlaysLocked()

	screens := s.Screens()
	s.screens = screens
	if len(screens) == 0 {
		return fmt.Errorf("no displays detected")
	}

	// Hide the main window so it does not appear in the capture.
	s.app.Hide()

	for _, sc := range screens {
		win := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:             fmt.Sprintf("wt-overlay-%d", sc.Index),
			URL:              fmt.Sprintf("/#/overlay?idx=%d", sc.Index),
			X:                sc.X,
			Y:                sc.Y,
			Width:            sc.Width,
			Height:           sc.Height,
			Frameless:        true,
			DisableResize:    true,
			AlwaysOnTop:      true,
			BackgroundType:   application.BackgroundTypeTransparent,
			BackgroundColour: application.NewRGBA(0, 0, 0, 0),
			InitialPosition:  application.WindowXY,
			HideOnEscape:     true,
		})
		s.overlays = append(s.overlays, win)
	}

	// Give the overlays a beat to paint, then tell them the geometry.
	s.app.Event.Emit(eventOverlayOpen, screens)
	return nil
}

// CommitRegion is called by an overlay when the user finishes a selection. sel
// is in global DIP coordinates. It closes the overlays, captures the region and
// emits the result (or an error) on the app event bus.
func (s *ScreenshotService) CommitRegion(sel screenshot.Rect, scale float64) {
	defer s.finishSession()

	if sel.Width < 1 || sel.Height < 1 {
		s.emit(EventShotCancel, nil)
		return
	}
	shot, b64, err := screenshot.CaptureRegion(sel, scale)
	if err != nil {
		s.emit(EventShotError, err.Error())
		return
	}
	s.mu.Lock()
	s.lastRect = sel
	s.lastScale = scale
	s.mu.Unlock()
	_ = shot
	s.emit(EventShotResult, ShotResult{
		Data:   b64,
		Width:  shot.Width,
		Height: shot.Height,
		Scale:  scale,
	})
}

// LastRegion returns the rectangle and scale of the most recent committed
// region selection, so the UI can re-capture the same area while scrolling.
func (s *ScreenshotService) LastRegion() (screenshot.Rect, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRect, s.lastScale
}

// CancelRegion dismisses the overlays without producing a capture.
func (s *ScreenshotService) CancelRegion() {
	s.finishSession()
	s.emit(EventShotCancel, nil)
}

// CapturePrimary grabs the primary display in full.
func (s *ScreenshotService) CapturePrimary() (ShotResult, error) {
	screens := s.Screens()
	var target *ScreenInfo
	for i := range screens {
		if screens[i].Primary {
			target = &screens[i]
			break
		}
	}
	if target == nil && len(screens) > 0 {
		target = &screens[0]
	}
	if target == nil {
		return ShotResult{}, fmt.Errorf("no displays detected")
	}
	return s.CaptureRect(screenshot.Rect{
		X: target.X, Y: target.Y, Width: target.Width, Height: target.Height,
	}, target.Scale)
}

// CaptureRect captures an explicit global-DIP rectangle (used by the "full
// screen" button and by scripted captures).
func (s *ScreenshotService) CaptureRect(r screenshot.Rect, scale float64) (ShotResult, error) {
	shot, b64, err := screenshot.CaptureRegion(r, scale)
	if err != nil {
		return ShotResult{}, err
	}
	return ShotResult{Data: b64, Width: shot.Width, Height: shot.Height, Scale: scale}, nil
}

// StitchFrames joins a list of base64 PNG frames into one tall image.
func (s *ScreenshotService) StitchFrames(framesB64 []string, mode string) (screenshot.StitchResult, error) {
	if len(framesB64) == 0 {
		return screenshot.StitchResult{}, fmt.Errorf("no frames supplied")
	}
	frames := make([]screenshot.Shot, 0, len(framesB64))
	for i, b64 := range framesB64 {
		raw, err := decodeBase64Image(b64)
		if err != nil {
			return screenshot.StitchResult{}, fmt.Errorf("frame %d: %w", i, err)
		}
		w, h := pngDims(raw)
		frames = append(frames, screenshot.Shot{Data: raw, Width: w, Height: h})
	}
	if strings.TrimSpace(mode) == "" {
		mode = screenshot.StitchModeScroll
	}
	return screenshot.Stitch(frames, mode)
}

// OCRText recognises text in a base64 PNG.
func (s *ScreenshotService) OCRText(dataB64, lang string) (screenshot.OCRResult, error) {
	raw, err := decodeBase64Image(dataB64)
	if err != nil {
		return screenshot.OCRResult{}, err
	}
	return screenshot.OCRImage(raw, lang)
}

// OCRRegion captures the given region and runs OCR over it in one step. Used by
// the "capture & recognise" flow so the image never needs to round-trip.
func (s *ScreenshotService) OCRRegion(r screenshot.Rect, scale float64, lang string) (screenshot.OCRResult, error) {
	shot, _, err := screenshot.CaptureRegion(r, scale)
	if err != nil {
		return screenshot.OCRResult{}, err
	}
	return screenshot.OCRImage(shot.Data, lang)
}

// SaveImage writes a base64 PNG to path (creating parents), returning the path.
// When path is empty a timestamped file is written inside the app data dir.
func (s *ScreenshotService) SaveImage(path, dataB64 string) (string, error) {
	raw, err := decodeBase64Image(dataB64)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		dir, err := config.AppDir()
		if err != nil {
			return "", err
		}
		shots := filepath.Join(dir, "screenshots")
		if err := os.MkdirAll(shots, 0o700); err != nil {
			return "", err
		}
		path = filepath.Join(shots, "screenshot-"+timestamp()+".png")
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// CopyToClipboard places a base64 PNG onto the system clipboard as an image.
func (s *ScreenshotService) CopyToClipboard(dataB64 string) error {
	raw, err := decodeBase64Image(dataB64)
	if err != nil {
		return err
	}
	return screenshot.CopyImageToClipboard(raw)
}

// ---- helpers ----

func (s *ScreenshotService) emit(name string, data any) {
	if s.app == nil {
		return
	}
	if data == nil {
		s.app.Event.Emit(name)
		return
	}
	s.app.Event.Emit(name, data)
}

// finishSession closes all overlays and restores the main window.
func (s *ScreenshotService) finishSession() {
	s.mu.Lock()
	s.closeOverlaysLocked()
	app := s.app
	s.mu.Unlock()
	if app != nil {
		app.Show()
	}
}

func (s *ScreenshotService) closeOverlaysLocked() {
	for _, w := range s.overlays {
		w.Close()
	}
	s.overlays = nil
}

// decodeBase64Image decodes a base64 string, tolerating a data-URI prefix and
// URL-safe encoding.
func decodeBase64Image(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ','); i >= 0 && strings.Contains(s[:i], "base64") {
		s = s[i+1:]
	}
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	if raw, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}

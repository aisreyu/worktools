package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/aisreyu/worktools/internal/clipboard"
	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// clipboardEventName is emitted whenever the stored history changes so open
// panels can refresh live.
const clipboardEventName = "nb:clipboard"

// clipboardPollInterval is how often the OS clipboard is checked for new text.
const clipboardPollInterval = 1500 * time.Millisecond

// ClipboardService is the CopyLess-style clipboard manager: it captures text,
// images, file paths and rich text copied anywhere in the OS, persists them
// encrypted, and exposes search, pin, edit, delete and export operations.
type ClipboardService struct {
	v *vault.Vault

	mu      sync.Mutex
	lastKey string
	lastSig int64
	stop    chan struct{}
	done    chan struct{}
}

// NewClipboardService wires the service to a vault.
func NewClipboardService(v *vault.Vault) *ClipboardService { return &ClipboardService{v: v} }

// ServiceName labels the service in logs.
func (s *ClipboardService) ServiceName() string { return "ClipboardService" }

// ServiceStartup launches the background clipboard poller.
func (s *ClipboardService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.watch()
	return nil
}

// ServiceShutdown stops the poller.
func (s *ClipboardService) ServiceShutdown() error {
	if s.stop != nil {
		close(s.stop)
		<-s.done
		s.stop = nil
	}
	return nil
}

// watch polls the OS clipboard and records new captures while auto-capture is
// on and the vault is unlocked.
func (s *ClipboardService) watch() {
	defer close(s.done)
	ticker := time.NewTicker(clipboardPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.captureOnce(false)
		}
	}
}

// captureOnce reads the clipboard once. When force is false it respects the
// auto-capture setting; either way it de-duplicates against the last seen
// capture. Content is read from the native pasteboard (text, images, file
// paths, rich text), falling back to the Wails text API when unavailable.
func (s *ClipboardService) captureOnce(force bool) {
	if s.v.Locked() {
		return
	}
	if !force && !s.v.ClipboardAutoCapture() {
		return
	}
	cap, sig := s.read()
	if cap.Kind == "" {
		return
	}
	key := cap.Key()

	s.mu.Lock()
	if key == s.lastKey && sig == s.lastSig && sig != 0 {
		s.mu.Unlock()
		return
	}
	s.lastKey, s.lastSig = key, sig
	s.mu.Unlock()

	it, err := s.v.SaveClipboardCapture(cap)
	if err != nil || it.ID == 0 {
		return
	}
	s.emit()
}

// read captures the current clipboard from the native pasteboard reader, using
// the Wails text API as a fallback. It returns the capture and the pasteboard
// change token (0 when not available).
func (s *ClipboardService) read() (clipboard.Capture, int64) {
	if cap, sig := clipboard.Read(); cap.Kind != "" {
		return cap, sig
	}
	app := application.Get()
	if app == nil || app.Clipboard == nil {
		return clipboard.Capture{}, 0
	}
	text, ok := app.Clipboard.Text()
	if !ok {
		return clipboard.Capture{}, 0
	}
	return clipboard.TextCapture(text), 0
}

// remember records the identity of a capture so the poller does not re-store it.
func (s *ClipboardService) remember(cap clipboard.Capture, sig int64) {
	s.mu.Lock()
	s.lastKey, s.lastSig = cap.Key(), sig
	s.mu.Unlock()
}

// ---- query ----

// List returns stored captures filtered by query substring and kind ("" = all).
func (s *ClipboardService) List(query, kind string) ([]store.ClipboardItem, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListClipboard(query, kind)
}

// Kinds returns the kinds present in the history, for filter chips.
func (s *ClipboardService) Kinds() ([]string, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.Kinds()
}

// ---- mutation ----

// Add stores a text capture immediately (used by the "capture now" button).
func (s *ClipboardService) Add(text string) (store.ClipboardItem, error) {
	if s.v.Locked() {
		return store.ClipboardItem{}, vault.ErrLocked
	}
	cap := clipboard.TextCapture(text)
	it, err := s.v.SaveClipboardCapture(cap)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	s.remember(cap, 0)
	if it.ID != 0 {
		s.emit()
	}
	return it, nil
}

// CaptureNow reads the current OS clipboard (text, image or file) and stores it
// regardless of the auto-capture setting.
func (s *ClipboardService) CaptureNow() (store.ClipboardItem, error) {
	if s.v.Locked() {
		return store.ClipboardItem{}, vault.ErrLocked
	}
	cap, sig := s.read()
	if cap.Kind == "" {
		return store.ClipboardItem{}, errText("clipboard is empty")
	}
	it, err := s.v.SaveClipboardCapture(cap)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	s.remember(cap, sig)
	if it.ID != 0 {
		s.emit()
	}
	return it, nil
}

// Copy puts an item back on the OS clipboard without triggering a re-capture.
// Image items are written as text (their descriptive label + hash) because the
// Wails clipboard API only supports text.
func (s *ClipboardService) Copy(id int64) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	items, err := s.v.ListClipboard("", "")
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.ID == id {
			if app := application.Get(); app != nil && app.Clipboard != nil {
				app.Clipboard.SetText(it.Text)
			}
			s.remember(clipboard.Capture{Kind: it.Kind, Text: it.Text}, 0)
			return nil
		}
	}
	return errText("clipboard item not found")
}

// GetImage returns an image capture's payload as a base64 PNG string, or "" for
// non-image items.
func (s *ClipboardService) GetImage(id int64) (string, error) {
	if s.v.Locked() {
		return "", vault.ErrLocked
	}
	return s.v.ClipboardData(id)
}

// ClearData erases a stored image payload, mirroring how secrets are wiped
// after copy. The entry's metadata is kept.
func (s *ClipboardService) ClearData(id int64) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if err := s.v.ClearClipboardData(id); err != nil {
		return err
	}
	s.emit()
	return nil
}

// UpdateText edits a capture in place.
func (s *ClipboardService) UpdateText(id int64, text string) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if strings.TrimSpace(text) == "" {
		return errText("clipboard text is required")
	}
	if err := s.v.UpdateClipboardText(id, text); err != nil {
		return err
	}
	s.remember(clipboard.TextCapture(text), 0)
	s.emit()
	return nil
}

// Delete removes one capture.
func (s *ClipboardService) Delete(id int64) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if err := s.v.DeleteClipboard(id); err != nil {
		return err
	}
	s.emit()
	return nil
}

// TogglePin pins or unpins a capture.
func (s *ClipboardService) TogglePin(id int64, pinned bool) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if err := s.v.SetClipboardPinned(id, pinned); err != nil {
		return err
	}
	s.emit()
	return nil
}

// Clear removes history. When keepPinned is true only unpinned items go.
func (s *ClipboardService) Clear(keepPinned bool) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if err := s.v.ClearClipboard(keepPinned); err != nil {
		return err
	}
	s.emit()
	return nil
}

// PurgeOlderThan drops unpinned captures older than the given number of days.
func (s *ClipboardService) PurgeOlderThan(days int) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if days <= 0 {
		return errText("days must be positive")
	}
	if err := s.v.PurgeClipboardOlderThan(int64(days) * 86400); err != nil {
		return err
	}
	s.emit()
	return nil
}

// AutoCapture reports whether new clipboard text is recorded automatically.
func (s *ClipboardService) AutoCapture() bool {
	if s.v.Locked() {
		return false
	}
	return s.v.ClipboardAutoCapture()
}

// SetAutoCapture persists the auto-capture flag.
func (s *ClipboardService) SetAutoCapture(on bool) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	return s.v.SetClipboardAutoCapture(on)
}

// ---- export ----

// ExportText renders the (optionally filtered) history as plain text, one
// capture per entry, ready to be written to a file by ExportService.SaveText.
func (s *ClipboardService) ExportText(query, kind string) (string, error) {
	items, err := s.List(query, kind)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# Clipboard history\n\n")
	for _, it := range items {
		mark := ""
		if it.Pinned {
			mark = " [pinned]"
		}
		body := it.Text
		if it.Kind == clipboard.KindImage {
			body = it.Preview
		}
		fmt.Fprintf(&b, "## %s%s (%s)\n\n%s\n\n",
			time.Unix(it.CreatedAt, 0).Format("2006-01-02 15:04:05"), mark, it.Kind, body)
	}
	return b.String(), nil
}

func (s *ClipboardService) emit() {
	if app := application.Get(); app != nil {
		app.Event.Emit(clipboardEventName)
	}
}

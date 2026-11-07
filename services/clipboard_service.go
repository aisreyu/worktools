package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/clipboard"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/vault"
)

// clipboardEventName is emitted whenever the stored history changes so open
// panels can refresh live.
const clipboardEventName = "nb:clipboard"

// clipboardPollInterval is how often the OS clipboard is checked for new text.
const clipboardPollInterval = 1500 * time.Millisecond

// ClipboardService is the CopyLess-style clipboard manager: it captures text
// copied anywhere in the OS, persists it encrypted, and exposes search, pin,
// edit, delete and export operations.
type ClipboardService struct {
	v *vault.Vault

	mu       sync.Mutex
	lastText string
	stop     chan struct{}
	done     chan struct{}
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
// auto-capture setting; either way it de-duplicates against the last seen text.
func (s *ClipboardService) captureOnce(force bool) {
	if s.v.Locked() {
		return
	}
	if !force && !s.v.ClipboardAutoCapture() {
		return
	}
	app := application.Get()
	if app == nil || app.Clipboard == nil {
		return
	}
	text, ok := app.Clipboard.Text()
	if !ok {
		return
	}
	text = clipboard.Normalize(text)
	if strings.TrimSpace(text) == "" {
		return
	}
	s.mu.Lock()
	if text == s.lastText {
		s.mu.Unlock()
		return
	}
	s.lastText = text
	s.mu.Unlock()

	it, err := s.v.SaveClipboard(text)
	if err != nil || it.ID == 0 {
		return
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(clipboardEventName)
	}
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
	it, err := s.v.SaveClipboard(text)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	s.mu.Lock()
	s.lastText = clipboard.Normalize(text)
	s.mu.Unlock()
	if it.ID != 0 {
		s.emit()
	}
	return it, nil
}

// CaptureNow reads the current OS clipboard and stores it regardless of the
// auto-capture setting.
func (s *ClipboardService) CaptureNow() (store.ClipboardItem, error) {
	if s.v.Locked() {
		return store.ClipboardItem{}, vault.ErrLocked
	}
	app := application.Get()
	if app == nil || app.Clipboard == nil {
		return store.ClipboardItem{}, errText("clipboard unavailable")
	}
	text, ok := app.Clipboard.Text()
	if !ok || strings.TrimSpace(text) == "" {
		return store.ClipboardItem{}, errText("clipboard is empty")
	}
	it, err := s.v.SaveClipboard(text)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	s.mu.Lock()
	s.lastText = clipboard.Normalize(text)
	s.mu.Unlock()
	if it.ID != 0 {
		s.emit()
	}
	return it, nil
}

// Copy puts an item's text back on the OS clipboard without re-capturing it.
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
			s.mu.Lock()
			s.lastText = clipboard.Normalize(it.Text)
			s.mu.Unlock()
			return nil
		}
	}
	return errText("clipboard item not found")
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
	s.mu.Lock()
	s.lastText = clipboard.Normalize(text)
	s.mu.Unlock()
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
		fmt.Fprintf(&b, "## %s%s (%s)\n\n%s\n\n",
			time.Unix(it.CreatedAt, 0).Format("2006-01-02 15:04:05"), mark, it.Kind, it.Text)
	}
	return b.String(), nil
}

func (s *ClipboardService) emit() {
	if app := application.Get(); app != nil {
		app.Event.Emit(clipboardEventName)
	}
}

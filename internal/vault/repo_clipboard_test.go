package vault

import (
	"strings"
	"testing"
)

func TestClipboardSaveListDedupe(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}

	it, err := v.SaveClipboard("hello world")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if it.ID == 0 || it.Kind != "text" {
		t.Fatalf("unexpected item: %+v", it)
	}

	// Same text again is dropped.
	dup, err := v.SaveClipboard("hello world")
	if err != nil {
		t.Fatalf("save dup: %v", err)
	}
	if dup.ID != 0 {
		t.Fatalf("expected duplicate to be skipped, got %+v", dup)
	}

	// Empty / whitespace-only captures are ignored.
	if got, _ := v.SaveClipboard("   \n  "); got.ID != 0 {
		t.Fatalf("expected blank capture to be skipped, got %+v", got)
	}

	if _, err := v.SaveClipboard("https://example.com/x"); err != nil {
		t.Fatalf("save link: %v", err)
	}

	list, err := v.ListClipboard("", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if list[0].Kind != "link" {
		t.Fatalf("newest should be the link, got %+v", list[0])
	}

	// Text must be encrypted at rest.
	var ct []byte
	if err := v.db.QueryRow(`SELECT text_ct FROM clipboard_items WHERE id = ?`, it.ID).Scan(&ct); err != nil {
		t.Fatalf("read text_ct: %v", err)
	}
	if strings.Contains(string(ct), "hello") {
		t.Fatal("clipboard text was not encrypted at rest")
	}

	// Substring + kind filters.
	if got, _ := v.ListClipboard("HELLO", ""); len(got) != 1 {
		t.Fatalf("query filter failed: %+v", got)
	}
	if got, _ := v.ListClipboard("", "link"); len(got) != 1 || got[0].Kind != "link" {
		t.Fatalf("kind filter failed: %+v", got)
	}
}

func TestClipboardPinUpdateDeleteClear(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	a, _ := v.SaveClipboard("first")
	b, _ := v.SaveClipboard("second")

	if err := v.SetClipboardPinned(a.ID, true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	list, _ := v.ListClipboard("", "")
	if list[0].ID != a.ID || !list[0].Pinned {
		t.Fatalf("pinned item should sort first: %+v", list)
	}

	if err := v.UpdateClipboardText(b.ID, "func x() {}\n"); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, _ = v.ListClipboard("", "")
	for _, it := range list {
		if it.ID == b.ID && it.Kind != "code" {
			t.Fatalf("update should re-detect kind, got %q", it.Kind)
		}
	}

	// Auto clear keeps pinned items; manual clear removes everything.
	if err := v.ClearClipboard(true); err != nil {
		t.Fatalf("auto clear: %v", err)
	}
	list, _ = v.ListClipboard("", "")
	if len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("auto clear should keep only pinned: %+v", list)
	}
	if err := v.ClearClipboard(false); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if list, _ = v.ListClipboard("", ""); len(list) != 0 {
		t.Fatalf("manual clear left items: %+v", list)
	}
}

func TestClipboardAutoCaptureSetting(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !v.ClipboardAutoCapture() {
		t.Fatal("auto-capture should default to on")
	}
	if err := v.SetClipboardAutoCapture(false); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v.ClipboardAutoCapture() {
		t.Fatal("auto-capture should be off after set")
	}
	var ct []byte
	if err := v.db.QueryRow(`SELECT value_ct FROM app_settings WHERE key = ?`, settingClipboardAuto).Scan(&ct); err != nil {
		t.Fatalf("read setting: %v", err)
	}
	if string(ct) == "0" {
		t.Fatal("setting value was not encrypted at rest")
	}
}

func TestClipboardLocked(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := v.SaveClipboard("secret"); err != nil {
		t.Fatalf("save: %v", err)
	}
	v.Lock()
	if _, err := v.ListClipboard("", ""); err != ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
	if _, err := v.SaveClipboard("x"); err != ErrLocked {
		t.Fatalf("expected ErrLocked on save, got %v", err)
	}
}

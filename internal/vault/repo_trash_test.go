package vault

import (
	"testing"

	"github.com/aisreyu/worktools/internal/store"
)

// TestTrashLifecycle verifies that deletes become soft deletes (hidden from the
// normal list), that trashed items surface in the recycle bin, and that both
// restore and permanent purge behave correctly.
func TestTrashLifecycle(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}

	id, err := v.SaveURL(store.URLEntry{Title: "Grafana", URL: "https://grafana.example.com"})
	if err != nil {
		t.Fatalf("save url: %v", err)
	}
	if err := v.DeleteURL(id); err != nil {
		t.Fatalf("delete url: %v", err)
	}

	// Hidden from the live list, present in the bin.
	urls, err := v.ListURLs()
	if err != nil {
		t.Fatalf("list urls: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("soft-deleted url still listed: %d", len(urls))
	}
	trash, err := v.ListTrash()
	if err != nil {
		t.Fatalf("list trash: %v", err)
	}
	if len(trash) != 1 || trash[0].Kind != "url" || trash[0].ID != id || trash[0].Title != "Grafana" {
		t.Fatalf("unexpected trash: %+v", trash)
	}

	// Restore brings it back.
	if err := v.Restore("url", id); err != nil {
		t.Fatalf("restore: %v", err)
	}
	urls, _ = v.ListURLs()
	if len(urls) != 1 {
		t.Fatalf("restored url missing, got %d", len(urls))
	}
	trash, _ = v.ListTrash()
	if len(trash) != 0 {
		t.Fatalf("trash not empty after restore: %+v", trash)
	}

	// Delete again, then purge permanently.
	if err := v.DeleteURL(id); err != nil {
		t.Fatalf("delete again: %v", err)
	}
	if err := v.Purge("url", id); err != nil {
		t.Fatalf("purge: %v", err)
	}
	trash, _ = v.ListTrash()
	if len(trash) != 0 {
		t.Fatalf("trash not empty after purge: %+v", trash)
	}

	// Unknown kind is rejected.
	if err := v.Restore("bogus", 1); err != errBadTrashKind {
		t.Fatalf("expected errBadTrashKind, got %v", err)
	}
}

// TestTrashEmptyAndRetention covers EmptyTrash and the age-based retention purge.
func TestTrashEmptyAndRetention(t *testing.T) {
	v, db := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}

	noteID, err := v.SaveNote(store.NoteEntry{Title: "runbook", Content: "restart api"})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	if err := v.DeleteNote(noteID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	// Backdate the deletion so the retention purge catches it.
	if _, err := db.Exec(`UPDATE note_entries SET deleted_at = ? WHERE id = ?`, 1000, noteID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	n, err := v.PurgeTrashOlderThan(24 * 3600)
	if err != nil {
		t.Fatalf("purge older: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 purged, got %d", n)
	}

	// Empty clears everything left behind.
	id2, _ := v.SaveURL(store.URLEntry{Title: "x", URL: "https://x.example.com"})
	_ = v.DeleteURL(id2)
	if err := v.EmptyTrash(); err != nil {
		t.Fatalf("empty: %v", err)
	}
	trash, _ := v.ListTrash()
	if len(trash) != 0 {
		t.Fatalf("trash not empty after empty: %+v", trash)
	}
}

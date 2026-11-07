package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateLegacyNoteEntries simulates a database created before the
// daily-use update (note_entries without sensitive/attach_* columns) and
// verifies Open back-fills the missing columns without losing data.
func TestMigrateLegacyNoteEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Build a v1-style database: note_entries lacks the new columns.
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	if _, err := legacy.Exec(`
		CREATE TABLE note_entries (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			title         TEXT    NOT NULL,
			content_ct    BLOB,
			content_nonce BLOB,
			pinned        INTEGER NOT NULL DEFAULT 0,
			created_at    INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL
		);
		INSERT INTO note_entries(title, content_ct, content_nonce, created_at, updated_at)
		VALUES ('legacy', X'00', X'00', 1, 1);
	`); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	for _, col := range []string{"sensitive", "attach_ct", "attach_nonce", "attach_name", "attach_mime"} {
		ok, err := hasColumn(db, "note_entries", col)
		if err != nil {
			t.Fatalf("hasColumn(%s): %v", col, err)
		}
		if !ok {
			t.Errorf("column %s missing after migration", col)
		}
	}

	// Existing row survives and gains the new column defaults.
	var title string
	var sensitive int
	var attachName string
	if err := db.QueryRow(`SELECT title, sensitive, attach_name FROM note_entries WHERE id = 1`).
		Scan(&title, &sensitive, &attachName); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if title != "legacy" || sensitive != 0 || attachName != "" {
		t.Errorf("unexpected row after migration: title=%q sensitive=%d attachName=%q", title, sensitive, attachName)
	}

	// Re-opening must be a no-op (idempotent).
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer db2.Close()
}

// TestMigrateFreshDatabase ensures a brand-new database needs no back-fill
// and still exposes every expected column.
func TestMigrateFreshDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	for _, col := range []string{"sensitive", "attach_ct", "attach_nonce", "attach_name", "attach_mime"} {
		ok, err := hasColumn(db, "note_entries", col)
		if err != nil {
			t.Fatalf("hasColumn(%s): %v", col, err)
		}
		if !ok {
			t.Errorf("column %s missing on fresh database", col)
		}
	}
}

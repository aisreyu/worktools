// Package store owns the SQLite database: opening it, migrating the schema and
// providing typed access to the non-secret metadata. Encrypted columns are
// handled by the vault package.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

//go:embed schema.sql
var schemaSQL string

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. Use ":memory:" for tests.
func Open(path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite is safe for concurrent use but SQLite prefers a single
	// writer; a small pool avoids "database is locked" under WAL.
	db.SetMaxOpenConns(4)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return db, nil
}

// migrate adds columns introduced after v1 to databases created by an older
// build. CREATE TABLE IF NOT EXISTS leaves pre-existing tables untouched, so
// columns must be back-filled explicitly.
func migrate(db *sql.DB) error {
	// note_entries gained sensitive + attachment columns in the daily-use
	// update; older databases lack them.
	adds := []struct{ table, column, ddl string }{
		{"note_entries", "sensitive", "ALTER TABLE note_entries ADD COLUMN sensitive INTEGER NOT NULL DEFAULT 0"},
		{"note_entries", "attach_ct", "ALTER TABLE note_entries ADD COLUMN attach_ct BLOB"},
		{"note_entries", "attach_nonce", "ALTER TABLE note_entries ADD COLUMN attach_nonce BLOB"},
		{"note_entries", "attach_name", "ALTER TABLE note_entries ADD COLUMN attach_name TEXT NOT NULL DEFAULT ''"},
		{"note_entries", "attach_mime", "ALTER TABLE note_entries ADD COLUMN attach_mime TEXT NOT NULL DEFAULT ''"},
	}
	for _, a := range adds {
		ok, err := hasColumn(db, a.table, a.column)
		if err != nil {
			return err
		}
		if !ok {
			if _, err := db.Exec(a.ddl); err != nil {
				return fmt.Errorf("add %s.%s: %w", a.table, a.column, err)
			}
		}
	}
	return nil
}

// hasColumn reports whether table already contains the named column.
func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid        int
			name, typ  string
			notNull    int
			dflt       sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

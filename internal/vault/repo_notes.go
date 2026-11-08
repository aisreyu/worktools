package vault

import (
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// ---- M4: note entries ----

// ListNotes returns all notes with content decrypted, pinned first.
func (v *Vault) ListNotes() ([]store.NoteEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, title, content_ct, content_nonce, pinned, sensitive, attach_ct, attach_nonce, attach_name, attach_mime, created_at, updated_at, deleted_at
        FROM note_entries WHERE deleted_at = 0 ORDER BY pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.NoteEntry
	for rows.Next() {
		var (
			e           store.NoteEntry
			ct, non     []byte
			attCT, attN []byte
			pinned      int
			sensitive   int
		)
		if err := rows.Scan(&e.ID, &e.Title, &ct, &non, &pinned, &sensitive, &attCT, &attN, &e.AttachmentName, &e.AttachmentMime, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt); err != nil {
			return nil, err
		}
		if e.Content, err = c.DecryptField(ct, non, aadNote); err != nil {
			return nil, err
		}
		if len(attCT) > 0 {
			if e.Attachment, err = c.DecryptField(attCT, attN, aadNoteAttach); err != nil {
				return nil, err
			}
		}
		e.Pinned = pinned != 0
		e.Sensitive = sensitive != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetNote returns a single note by id.
func (v *Vault) GetNote(id int64) (store.NoteEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.NoteEntry{}, err
	}
	var (
		e           store.NoteEntry
		ct, non     []byte
		attCT, attN []byte
		pinned      int
		sensitive   int
	)
	err = v.db.QueryRow(`SELECT id, title, content_ct, content_nonce, pinned, sensitive, attach_ct, attach_nonce, attach_name, attach_mime, created_at, updated_at
        FROM note_entries WHERE id = ?`, id).
		Scan(&e.ID, &e.Title, &ct, &non, &pinned, &sensitive, &attCT, &attN, &e.AttachmentName, &e.AttachmentMime, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return store.NoteEntry{}, err
	}
	if e.Content, err = c.DecryptField(ct, non, aadNote); err != nil {
		return store.NoteEntry{}, err
	}
	if len(attCT) > 0 {
		if e.Attachment, err = c.DecryptField(attCT, attN, aadNoteAttach); err != nil {
			return store.NoteEntry{}, err
		}
	}
	e.Pinned = pinned != 0
	e.Sensitive = sensitive != 0
	return e, nil
}

// SaveNote inserts (ID==0) or updates a note.
func (v *Vault) SaveNote(e store.NoteEntry) (int64, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return 0, err
	}
	ct, non, err := c.EncryptField(e.Content, aadNote)
	if err != nil {
		return 0, err
	}
	var attCT, attN []byte
	if e.Attachment != "" {
		if attCT, attN, err = c.EncryptField(e.Attachment, aadNoteAttach); err != nil {
			return 0, err
		}
	}
	pinned := 0
	if e.Pinned {
		pinned = 1
	}
	sensitive := 0
	if e.Sensitive {
		sensitive = 1
	}
	now := time.Now().Unix()
	if e.ID == 0 {
		res, err := v.db.Exec(`INSERT INTO note_entries
            (title, content_ct, content_nonce, pinned, sensitive, attach_ct, attach_nonce, attach_name, attach_mime, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, ct, non, pinned, sensitive, attCT, attN, e.AttachmentName, e.AttachmentMime, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = v.db.Exec(`UPDATE note_entries SET title=?, content_ct=?, content_nonce=?, pinned=?, sensitive=?, attach_ct=?, attach_nonce=?, attach_name=?, attach_mime=?, updated_at=? WHERE id=?`,
		e.Title, ct, non, pinned, sensitive, attCT, attN, e.AttachmentName, e.AttachmentMime, now, e.ID)
	return e.ID, err
}

// DeleteNote moves a note to the trash (soft delete).
func (v *Vault) DeleteNote(id int64) error {
	_, err := v.db.Exec(`UPDATE note_entries SET deleted_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

// noteRow mirrors note_entries for backup/restore.
type noteRow struct {
	ID        int64
	Title     string
	Content   string
	Pinned    bool
	CreatedAt int64
	UpdatedAt int64
}

// listNoteRows returns raw decrypted notes for backup.
func (v *Vault) listNoteRows() ([]noteRow, error) {
	entries, err := v.ListNotes()
	if err != nil {
		return nil, err
	}
	out := make([]noteRow, 0, len(entries))
	for _, e := range entries {
		out = append(out, noteRow{e.ID, e.Title, e.Content, e.Pinned, e.CreatedAt, e.UpdatedAt})
	}
	return out, nil
}

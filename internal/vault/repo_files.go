package vault

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/aisreyu/worktools/internal/config"
	"github.com/aisreyu/worktools/internal/store"
)

// ---- M7: private-folder files ----

// inlineMaxBytes is the largest file body kept inside SQLite. Larger bodies are
// written to an encrypted file on disk so the database stays compact.
const inlineMaxBytes = 512 * 1024

// ErrEmptyFile is returned when a save carries no file body.
var ErrEmptyFile = errors.New("file data is required")

// filesDir returns (creating it if needed) the directory that holds encrypted
// large-file blobs.
func filesDir() (string, error) {
	dir, err := config.AppDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "files")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ListFiles returns file metadata (no bodies) newest first.
func (v *Vault) ListFiles() ([]store.FileEntry, error) {
	if _, err := v.cryptoOrErr(); err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, title, category, note, mime, size, created_at, updated_at
        FROM file_entries ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.FileEntry
	for rows.Next() {
		var e store.FileEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.Category, &e.Note, &e.Mime, &e.Size, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetFile returns a single file with its body decrypted into Data (base64).
func (v *Vault) GetFile(id int64) (store.FileEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.FileEntry{}, err
	}
	var (
		e       store.FileEntry
		mode    string
		path    string
		ct, non []byte
	)
	err = v.db.QueryRow(`SELECT id, title, category, note, mime, size, stored, data_ct, data_nonce, disk_path, created_at, updated_at
        FROM file_entries WHERE id = ?`, id).
		Scan(&e.ID, &e.Title, &e.Category, &e.Note, &e.Mime, &e.Size, &mode, &ct, &non, &path, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return store.FileEntry{}, err
	}
	if mode == "disk" {
		if e.Data, err = readDiskBody(c, path); err != nil {
			return store.FileEntry{}, err
		}
	} else if len(ct) > 0 {
		if e.Data, err = c.DecryptField(ct, non, aadFileData); err != nil {
			return store.FileEntry{}, err
		}
	}
	return e, nil
}

// SaveFile inserts (ID==0) or updates a file entry. When Data is empty on an
// update, only the metadata changes and the stored body is preserved.
func (v *Vault) SaveFile(e store.FileEntry) (int64, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()

	if e.ID == 0 {
		if e.Data == "" {
			return 0, ErrEmptyFile
		}
		mode, ct, non, path, err := v.storeBody(c, e.Data)
		if err != nil {
			return 0, err
		}
		res, err := v.db.Exec(`INSERT INTO file_entries
            (title, category, note, mime, size, stored, data_ct, data_nonce, disk_path, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, e.Category, e.Note, e.Mime, e.Size, mode, ct, non, path, now, now)
		if err != nil {
			removeIfSet(path)
			return 0, err
		}
		return res.LastInsertId()
	}

	// Update: remember the previous on-disk blob so it can be cleaned up when
	// the body is replaced or the storage mode changes.
	var oldMode, oldPath string
	if err := v.db.QueryRow(`SELECT stored, disk_path FROM file_entries WHERE id = ?`, e.ID).
		Scan(&oldMode, &oldPath); err != nil {
		return 0, err
	}
	if e.Data == "" {
		_, err := v.db.Exec(`UPDATE file_entries SET title=?, category=?, note=?, updated_at=? WHERE id=?`,
			e.Title, e.Category, e.Note, now, e.ID)
		return e.ID, err
	}
	mode, ct, non, path, err := v.storeBody(c, e.Data)
	if err != nil {
		return 0, err
	}
	_, err = v.db.Exec(`UPDATE file_entries SET title=?, category=?, note=?, mime=?, size=?, stored=?, data_ct=?, data_nonce=?, disk_path=?, updated_at=? WHERE id=?`,
		e.Title, e.Category, e.Note, e.Mime, e.Size, mode, ct, non, path, now, e.ID)
	if err != nil {
		removeIfSet(path)
		return 0, err
	}
	if oldMode == "disk" && oldPath != "" && oldPath != path {
		_ = os.Remove(oldPath)
	}
	return e.ID, nil
}

// DeleteFile removes a file entry and any on-disk blob it owns.
func (v *Vault) DeleteFile(id int64) error {
	var mode, path string
	if err := v.db.QueryRow(`SELECT stored, disk_path FROM file_entries WHERE id = ?`, id).
		Scan(&mode, &path); err != nil {
		return err
	}
	if _, err := v.db.Exec(`DELETE FROM file_entries WHERE id = ?`, id); err != nil {
		return err
	}
	if mode == "disk" {
		removeIfSet(path)
	}
	return nil
}

// listFileRows returns every file entry with its body decrypted, for backup.
// Bodies larger than the backup cap are skipped so a backup stays manageable.
func (v *Vault) listFileRows() ([]store.FileEntry, error) {
	const backupMaxBytes = 16 * 1024 * 1024
	metas, err := v.ListFiles()
	if err != nil {
		return nil, err
	}
	out := make([]store.FileEntry, 0, len(metas))
	for _, m := range metas {
		if m.Size > backupMaxBytes {
			continue
		}
		full, err := v.GetFile(m.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, full)
	}
	return out, nil
}

// storeBody seals the base64 body and decides between inline and on-disk
// storage. Disk mode returns the path of the encrypted blob (ct/nonce empty).
func (v *Vault) storeBody(c *Crypto, data string) (mode string, ct, nonce []byte, path string, err error) {
	raw, derr := base64.StdEncoding.DecodeString(data)
	if derr != nil {
		return "", nil, nil, "", derr
	}
	if len(raw) <= inlineMaxBytes {
		ct, nonce, err = c.EncryptField(data, aadFileData)
		return "db", ct, nonce, "", err
	}
	sealed, sn, err := c.seal([]byte(data), aadFileData)
	if err != nil {
		return "", nil, nil, "", err
	}
	dir, err := filesDir()
	if err != nil {
		return "", nil, nil, "", err
	}
	name, err := randomName()
	if err != nil {
		return "", nil, nil, "", err
	}
	p := filepath.Join(dir, name)
	buf := make([]byte, 0, len(sn)+len(sealed))
	buf = append(buf, sn...)
	buf = append(buf, sealed...)
	if err := os.WriteFile(p, buf, 0o600); err != nil {
		return "", nil, nil, "", err
	}
	return "disk", nil, nil, p, nil
}

// readDiskBody reads and decrypts an on-disk blob (layout: nonce || ciphertext).
func readDiskBody(c *Crypto, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	ns := c.gcm.NonceSize()
	if len(b) < ns {
		return "", errors.New("corrupt private file")
	}
	pt, err := c.open(b[ns:], b[:ns], aadFileData)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// randomName returns a random hex filename for an on-disk blob.
func randomName() (string, error) {
	b, err := newRandom(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ".enc", nil
}

// removeIfSet deletes a path when it is non-empty, ignoring errors.
func removeIfSet(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

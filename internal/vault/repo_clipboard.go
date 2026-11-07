package vault

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/clipboard"
	"github.com/aisreyu/worktools/internal/store"
)

// ---- M9: persistent, manageable clipboard history ----

// ClipboardMax caps how many unpinned items are retained. Pinned items are
// never evicted by the cap.
const ClipboardMax = 500

// settingClipboardAuto is the app_settings key holding the auto-capture flag.
const settingClipboardAuto = "clipboard.autoCapture"

// ListClipboard returns stored captures, newest first, optionally filtered by a
// case-insensitive substring of the text and by kind ("" or "all" = any kind).
func (v *Vault) ListClipboard(query, kind string) ([]store.ClipboardItem, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, text_ct, text_nonce, kind, pinned, created_at, updated_at
        FROM clipboard_items ORDER BY pinned DESC, created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]store.ClipboardItem, 0, 64)
	for rows.Next() {
		var (
			it        store.ClipboardItem
			ct, nonce []byte
		)
		if err := rows.Scan(&it.ID, &ct, &nonce, &it.Kind, &it.Pinned, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		if it.Text, err = c.DecryptField(ct, nonce, aadClipText); err != nil {
			return nil, err
		}
		if kind != "" && kind != "all" && it.Kind != kind {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(it.Text), query) {
			continue
		}
		it.Preview = clipboard.Preview(it.Text)
		out = append(out, it)
	}
	return out, rows.Err()
}

// Kinds returns the distinct kinds currently present, for building filters.
func (v *Vault) Kinds() ([]string, error) {
	if _, err := v.cryptoOrErr(); err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT DISTINCT kind FROM clipboard_items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		if k != "" {
			seen[k] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// SaveClipboard stores a new capture. It normalises the text, skips empty and
// duplicate-of-newest captures (returning a zero item in that case) and evicts
// the oldest unpinned entries beyond ClipboardMax.
func (v *Vault) SaveClipboard(text string) (store.ClipboardItem, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.ClipboardItem{}, err
	}
	text = clipboard.Normalize(text)
	if strings.TrimSpace(text) == "" {
		return store.ClipboardItem{}, nil
	}
	// Dedupe against the most recent entry regardless of pin state.
	var (
		lastID   int64
		lastCt   []byte
		lastNonc []byte
		hasLast  bool
	)
	row := v.db.QueryRow(`SELECT id, text_ct, text_nonce FROM clipboard_items
        ORDER BY created_at DESC, id DESC LIMIT 1`)
	if err := row.Scan(&lastID, &lastCt, &lastNonc); err == nil {
		hasLast = true
	} else if err != sql.ErrNoRows {
		return store.ClipboardItem{}, err
	}
	if hasLast {
		lastText, err := c.DecryptField(lastCt, lastNonc, aadClipText)
		if err != nil {
			return store.ClipboardItem{}, err
		}
		if lastText == text {
			return store.ClipboardItem{}, nil
		}
	}

	ct, nonce, err := c.EncryptField(text, aadClipText)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	now := time.Now().Unix()
	kind := clipboard.DetectKind(text)
	res, err := v.db.Exec(`INSERT INTO clipboard_items
        (text_ct, text_nonce, kind, pinned, created_at, updated_at)
        VALUES (?, ?, ?, 0, ?, ?)`, ct, nonce, kind, now, now)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	id, _ := res.LastInsertId()
	if err := v.trimClipboard(); err != nil {
		return store.ClipboardItem{}, err
	}
	return store.ClipboardItem{ID: id, Text: text, Preview: clipboard.Preview(text),
		Kind: kind, CreatedAt: now, UpdatedAt: now}, nil
}

// trimClipboard drops the oldest unpinned entries beyond the cap.
func (v *Vault) trimClipboard() error {
	_, err := v.db.Exec(`DELETE FROM clipboard_items WHERE pinned = 0 AND id NOT IN (
        SELECT id FROM clipboard_items WHERE pinned = 0
        ORDER BY created_at DESC, id DESC LIMIT ?)`, ClipboardMax)
	return err
}

// UpdateClipboardText replaces the text of an existing capture, re-detecting
// its kind.
func (v *Vault) UpdateClipboardText(id int64, text string) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	text = clipboard.Normalize(text)
	if strings.TrimSpace(text) == "" {
		return errors.New("clipboard text is required")
	}
	ct, nonce, err := c.EncryptField(text, aadClipText)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`UPDATE clipboard_items SET text_ct=?, text_nonce=?, kind=?, updated_at=? WHERE id=?`,
		ct, nonce, clipboard.DetectKind(text), time.Now().Unix(), id)
	return err
}

// DeleteClipboard removes a single capture.
func (v *Vault) DeleteClipboard(id int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	_, err := v.db.Exec(`DELETE FROM clipboard_items WHERE id = ?`, id)
	return err
}

// SetClipboardPinned pins or unpins a capture so it survives clearing.
func (v *Vault) SetClipboardPinned(id int64, pinned bool) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	_, err := v.db.Exec(`UPDATE clipboard_items SET pinned=?, updated_at=? WHERE id=?`,
		pinned, time.Now().Unix(), id)
	return err
}

// ClearClipboard removes history. When auto is true (auto-capture overflow)
// pinned items are kept; a manual clear removes everything.
func (v *Vault) ClearClipboard(auto bool) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	if auto {
		_, err := v.db.Exec(`DELETE FROM clipboard_items WHERE pinned = 0`)
		return err
	}
	_, err := v.db.Exec(`DELETE FROM clipboard_items`)
	return err
}

// PurgeClipboardOlderThan drops unpinned captures older than the given age.
func (v *Vault) PurgeClipboardOlderThan(seconds int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second).Unix()
	_, err := v.db.Exec(`DELETE FROM clipboard_items WHERE pinned = 0 AND created_at < ?`, cutoff)
	return err
}

// ClipboardAutoCapture reports whether new clipboard captures are recorded
// automatically. Defaults to true on first read.
func (v *Vault) ClipboardAutoCapture() bool {
	c, err := v.cryptoOrErr()
	if err != nil {
		return false
	}
	var ct, nonce []byte
	err = v.db.QueryRow(`SELECT value_ct, value_nonce FROM app_settings WHERE key = ?`,
		settingClipboardAuto).Scan(&ct, &nonce)
	if err == sql.ErrNoRows {
		return true
	}
	if err != nil {
		return false
	}
	val, err := c.DecryptField(ct, nonce, aadClipSetting)
	if err != nil {
		return false
	}
	return val == "1"
}

// SetClipboardAutoCapture persists the auto-capture flag.
func (v *Vault) SetClipboardAutoCapture(on bool) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	val := "0"
	if on {
		val = "1"
	}
	ct, nonce, err := c.EncryptField(val, aadClipSetting)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`INSERT INTO app_settings (key, value_ct, value_nonce, updated_at)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(key) DO UPDATE SET value_ct=excluded.value_ct,
            value_nonce=excluded.value_nonce, updated_at=excluded.updated_at`,
		settingClipboardAuto, ct, nonce, time.Now().Unix())
	return err
}

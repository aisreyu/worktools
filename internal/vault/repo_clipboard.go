package vault

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
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
// Image entries carry their base64 PNG payload in Data so the UI can preview
// them inline.
func (v *Vault) ListClipboard(query, kind string) ([]store.ClipboardItem, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, text_ct, text_nonce, data_ct, data_nonce,
            data_mime, data_size, kind, pinned, created_at, updated_at
        FROM clipboard_items ORDER BY pinned DESC, created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]store.ClipboardItem, 0, 64)
	for rows.Next() {
		var (
			it                  store.ClipboardItem
			textCt, textNonce   []byte
			dataCt, dataNonce   []byte
			mime                string
		)
		if err := rows.Scan(&it.ID, &textCt, &textNonce, &dataCt, &dataNonce,
			&mime, &it.Size, &it.Kind, &it.Pinned, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		if it.Text, err = c.DecryptField(textCt, textNonce, aadClipText); err != nil {
			return nil, err
		}
		if len(dataCt) > 0 {
			raw, err := c.DecryptField(dataCt, dataNonce, aadClipData)
			if err != nil {
				return nil, err
			}
			it.Data = raw
			it.Mime = mime
		}
		if kind != "" && kind != "all" && it.Kind != kind {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(it.Text), query) {
			continue
		}
		it.Preview = clipboard.PreviewFor(it.Kind, it.Text)
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

// SaveClipboard stores a new text capture. It normalises the text, skips empty
// and duplicate-of-newest captures (returning a zero item in that case) and
// evicts the oldest unpinned entries beyond ClipboardMax.
func (v *Vault) SaveClipboard(text string) (store.ClipboardItem, error) {
	return v.SaveClipboardCapture(clipboard.TextCapture(text))
}

// SaveClipboardCapture stores a captured clipboard reading. Text and file kinds
// persist their text; images persist a base64 PNG payload. Empty captures and
// copies identical to the newest entry are skipped, returning a zero item.
func (v *Vault) SaveClipboardCapture(cap clipboard.Capture) (store.ClipboardItem, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.ClipboardItem{}, err
	}
	if cap.Kind == "" {
		cap.Kind = clipboard.KindText
	}
	cap.Text = clipboard.Normalize(cap.Text)
	if cap.Kind != clipboard.KindImage && strings.TrimSpace(cap.Text) == "" {
		return store.ClipboardItem{}, nil
	}
	if cap.Kind == clipboard.KindImage && len(cap.Data) == 0 {
		return store.ClipboardItem{}, nil
	}

	sum := sha256.Sum256(cap.Data)
	dataSHA := ""
	if len(cap.Data) > 0 {
		dataSHA = hex.EncodeToString(sum[:])
	}

	// Dedupe against the most recent entry regardless of pin state.
	var (
		lastID       int64
		lastTextCt   []byte
		lastTextNonc []byte
		lastDataSHA  string
		hasLast      bool
	)
	row := v.db.QueryRow(`SELECT id, text_ct, text_nonce, data_sha FROM clipboard_items
        ORDER BY created_at DESC, id DESC LIMIT 1`)
	if err := row.Scan(&lastID, &lastTextCt, &lastTextNonc, &lastDataSHA); err == nil {
		hasLast = true
	} else if err != sql.ErrNoRows {
		return store.ClipboardItem{}, err
	}
	if hasLast {
		if dataSHA != "" {
			if lastDataSHA == dataSHA {
				return store.ClipboardItem{}, nil
			}
		} else {
			lastText, err := c.DecryptField(lastTextCt, lastTextNonc, aadClipText)
			if err != nil {
				return store.ClipboardItem{}, err
			}
			if lastText == cap.Text {
				return store.ClipboardItem{}, nil
			}
		}
	}

	textCT, textNonce, err := c.EncryptField(cap.Text, aadClipText)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	var (
		dataCT    []byte
		dataNonce []byte
		mime      string
		size      int64
	)
	if len(cap.Data) > 0 {
		enc, nonce, err := c.EncryptField(base64.StdEncoding.EncodeToString(cap.Data), aadClipData)
		if err != nil {
			return store.ClipboardItem{}, err
		}
		dataCT, dataNonce = enc, nonce
		mime = cap.Mime
		if mime == "" {
			mime = "image/png"
		}
		size = int64(len(cap.Data))
	}

	now := time.Now().Unix()
	kind := cap.Kind
	res, err := v.db.Exec(`INSERT INTO clipboard_items
        (text_ct, text_nonce, data_ct, data_nonce, data_mime, data_sha, data_size,
         kind, pinned, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		textCT, textNonce, dataCT, dataNonce, mime, dataSHA, size, kind, now, now)
	if err != nil {
		return store.ClipboardItem{}, err
	}
	id, _ := res.LastInsertId()
	if err := v.trimClipboard(); err != nil {
		return store.ClipboardItem{}, err
	}
	item := store.ClipboardItem{ID: id, Text: cap.Text, Kind: kind,
		Preview: clipboard.PreviewFor(kind, cap.Text), Mime: mime, Size: size,
		CreatedAt: now, UpdatedAt: now}
	if len(cap.Data) > 0 {
		item.Data = base64.StdEncoding.EncodeToString(cap.Data)
	}
	return item, nil
}

// trimClipboard drops the oldest unpinned entries beyond the cap.
func (v *Vault) trimClipboard() error {
	_, err := v.db.Exec(`DELETE FROM clipboard_items WHERE pinned = 0 AND id NOT IN (
        SELECT id FROM clipboard_items WHERE pinned = 0
        ORDER BY created_at DESC, id DESC LIMIT ?)`, ClipboardMax)
	return err
}

// UpdateClipboardText replaces the text of an existing capture, re-detecting
// its kind. Any image payload is dropped because the entry is now text.
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
	_, err = v.db.Exec(`UPDATE clipboard_items
        SET text_ct=?, text_nonce=?, kind=?, data_ct=NULL, data_nonce=NULL,
            data_mime='', data_sha='', data_size=0, updated_at=? WHERE id=?`,
		ct, nonce, clipboard.DetectKind(text), time.Now().Unix(), id)
	return err
}

// ClipboardData returns the base64-encoded payload (a PNG for images) of a
// capture, or "" when the entry has no binary data.
func (v *Vault) ClipboardData(id int64) (string, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return "", err
	}
	var ct, nonce []byte
	err = v.db.QueryRow(`SELECT data_ct, data_nonce FROM clipboard_items WHERE id=?`, id).
		Scan(&ct, &nonce)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return c.DecryptField(ct, nonce, aadClipData)
}

// ClearClipboardData erases the stored binary payload for a capture while
// keeping its metadata, mirroring how secret fields are wiped after copy.
func (v *Vault) ClearClipboardData(id int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	_, err := v.db.Exec(`UPDATE clipboard_items
        SET data_ct=NULL, data_nonce=NULL, data_mime='', data_sha='', data_size=0,
            updated_at=? WHERE id=?`, time.Now().Unix(), id)
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

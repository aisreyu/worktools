package vault

import (
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// ---- M8: user-managed private-folder categories ----

// ErrCategoryInUse is returned when a category still has files attached.
var ErrCategoryInUse = errors.New("category is in use")

// ErrCategoryName is returned when a category name is blank.
var ErrCategoryName = errors.New("category name is required")

// defaultCategories seeds the built-in categories on first use. The codes are
// stable and match the historic labels; files keep referring to the code, so
// renaming a category never orphans the documents in it.
var defaultCategories = []struct{ code, name string }{
	{"id", "身份证件"},
	{"licence", "营业执照"},
	{"contract", "合同协议"},
	{"cert", "证书资质"},
	{"finance", "财务凭证"},
	{"other", "其他"},
}

// EnsureCategoryDefaults inserts the built-in categories the first time the
// vault is unlocked with an empty category table. It never re-seeds once the
// user has added or removed categories.
func (v *Vault) EnsureCategoryDefaults() error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	var n int
	if err := v.db.QueryRow(`SELECT COUNT(1) FROM file_categories`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now().Unix()
	for i, d := range defaultCategories {
		ct, nonce, err := c.EncryptField(d.name, aadFileCatName)
		if err != nil {
			return err
		}
		if _, err := v.db.Exec(`INSERT INTO file_categories
            (code, name_ct, name_nonce, sort_order, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?)`, d.code, ct, nonce, i, now, now); err != nil {
			return err
		}
	}
	return nil
}

// ListCategories returns categories ordered for display, names decrypted.
func (v *Vault) ListCategories() ([]store.FileCategory, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, code, name_ct, name_nonce, sort_order
        FROM file_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]store.FileCategory, 0, len(defaultCategories))
	for rows.Next() {
		var (
			cat      store.FileCategory
			ct, nonc []byte
		)
		if err := rows.Scan(&cat.ID, &cat.Code, &ct, &nonc, &cat.SortOrder); err != nil {
			return nil, err
		}
		if cat.Name, err = c.DecryptField(ct, nonc, aadFileCatName); err != nil {
			return nil, err
		}
		out = append(out, cat)
	}
	return out, rows.Err()
}

// CreateCategory adds a category with a freshly generated slug code.
func (v *Vault) CreateCategory(name string) (store.FileCategory, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.FileCategory{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return store.FileCategory{}, ErrCategoryName
	}
	code, err := newCategoryCode()
	if err != nil {
		return store.FileCategory{}, err
	}
	var maxOrder int
	_ = v.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM file_categories`).Scan(&maxOrder)

	ct, nonce, err := c.EncryptField(name, aadFileCatName)
	if err != nil {
		return store.FileCategory{}, err
	}
	now := time.Now().Unix()
	res, err := v.db.Exec(`INSERT INTO file_categories
        (code, name_ct, name_nonce, sort_order, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)`, code, ct, nonce, maxOrder+1, now, now)
	if err != nil {
		return store.FileCategory{}, err
	}
	id, _ := res.LastInsertId()
	return store.FileCategory{ID: id, Code: code, Name: name, SortOrder: maxOrder + 1}, nil
}

// RenameCategory updates the display name, leaving the code (and therefore any
// existing files) untouched.
func (v *Vault) RenameCategory(id int64, name string) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrCategoryName
	}
	ct, nonce, err := c.EncryptField(name, aadFileCatName)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`UPDATE file_categories SET name_ct = ?, name_nonce = ?, updated_at = ? WHERE id = ?`,
		ct, nonce, time.Now().Unix(), id)
	return err
}

// DeleteCategory removes a category. It refuses when files still reference it
// so documents never end up in a category that no longer exists.
func (v *Vault) DeleteCategory(id int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	var code string
	if err := v.db.QueryRow(`SELECT code FROM file_categories WHERE id = ?`, id).Scan(&code); err != nil {
		return err
	}
	var n int
	if err := v.db.QueryRow(`SELECT COUNT(1) FROM file_entries WHERE category = ? AND deleted_at = 0`, code).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrCategoryInUse
	}
	_, err := v.db.Exec(`DELETE FROM file_categories WHERE id = ?`, id)
	return err
}

// listCategoryRows returns every category with its name decrypted, for backup.
func (v *Vault) listCategoryRows() ([]store.FileCategory, error) {
	return v.ListCategories()
}

// restoreCategoryRows re-inserts categories from a backup, keeping their codes
// so imported files still resolve. Codes that already exist are left alone.
func (v *Vault) restoreCategoryRows(cats []store.FileCategory) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	for i, cat := range cats {
		if strings.TrimSpace(cat.Code) == "" {
			continue
		}
		var exists int
		if err := v.db.QueryRow(`SELECT COUNT(1) FROM file_categories WHERE code = ?`, cat.Code).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		ct, nonce, err := c.EncryptField(cat.Name, aadFileCatName)
		if err != nil {
			return err
		}
		order := cat.SortOrder
		if order == 0 {
			order = i
		}
		if _, err := v.db.Exec(`INSERT INTO file_categories
            (code, name_ct, name_nonce, sort_order, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?)`, cat.Code, ct, nonce, order, now, now); err != nil {
			return err
		}
	}
	return nil
}

// newCategoryCode returns a short random slug for a new category.
func newCategoryCode() (string, error) {
	b, err := newRandom(4)
	if err != nil {
		return "", err
	}
	return "c" + hex.EncodeToString(b), nil
}

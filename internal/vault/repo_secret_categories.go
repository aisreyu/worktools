package vault

import (
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// ---- user-managed secret-vault categories ----

// ErrSecretCategoryInUse is returned when a category still has secrets attached.
var ErrSecretCategoryInUse = ErrCategoryInUse

// defaultSecretCategories seeds the built-in categories on first use. Codes are
// stable and match the historic labels so existing secrets keep resolving after
// a category is renamed.
var defaultSecretCategories = []struct{ code, name string }{
	{"server", "服务器"},
	{"database", "数据库"},
	{"cloud", "云平台"},
	{"other", "其他"},
}

// EnsureSecretCategoryDefaults inserts the built-in categories the first time
// the vault is unlocked with an empty secret_categories table.
func (v *Vault) EnsureSecretCategoryDefaults() error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	var n int
	if err := v.db.QueryRow(`SELECT COUNT(1) FROM secret_categories`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now().Unix()
	for i, d := range defaultSecretCategories {
		ct, nonce, err := c.EncryptField(d.name, aadSecretCatName)
		if err != nil {
			return err
		}
		if _, err := v.db.Exec(`INSERT INTO secret_categories
            (code, name_ct, name_nonce, sort_order, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?)`, d.code, ct, nonce, i, now, now); err != nil {
			return err
		}
	}
	return nil
}

// ListSecretCategories returns categories ordered for display, names decrypted.
func (v *Vault) ListSecretCategories() ([]store.Category, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, code, name_ct, name_nonce, sort_order
        FROM secret_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]store.Category, 0, len(defaultSecretCategories))
	for rows.Next() {
		var (
			cat      store.Category
			ct, nonc []byte
		)
		if err := rows.Scan(&cat.ID, &cat.Code, &ct, &nonc, &cat.SortOrder); err != nil {
			return nil, err
		}
		if cat.Name, err = c.DecryptField(ct, nonc, aadSecretCatName); err != nil {
			return nil, err
		}
		out = append(out, cat)
	}
	return out, rows.Err()
}

// CreateSecretCategory adds a category with a freshly generated slug code.
func (v *Vault) CreateSecretCategory(name string) (store.Category, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.Category{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Category{}, ErrCategoryName
	}
	code, err := newCategoryCode()
	if err != nil {
		return store.Category{}, err
	}
	var maxOrder int
	_ = v.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM secret_categories`).Scan(&maxOrder)

	ct, nonce, err := c.EncryptField(name, aadSecretCatName)
	if err != nil {
		return store.Category{}, err
	}
	now := time.Now().Unix()
	res, err := v.db.Exec(`INSERT INTO secret_categories
        (code, name_ct, name_nonce, sort_order, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)`, code, ct, nonce, maxOrder+1, now, now)
	if err != nil {
		return store.Category{}, err
	}
	id, _ := res.LastInsertId()
	return store.Category{ID: id, Code: code, Name: name, SortOrder: maxOrder + 1}, nil
}

// RenameSecretCategory updates the display name, leaving the code (and any
// existing secrets) untouched.
func (v *Vault) RenameSecretCategory(id int64, name string) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrCategoryName
	}
	ct, nonce, err := c.EncryptField(name, aadSecretCatName)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`UPDATE secret_categories SET name_ct = ?, name_nonce = ?, updated_at = ? WHERE id = ?`,
		ct, nonce, time.Now().Unix(), id)
	return err
}

// DeleteSecretCategory removes a category. It refuses when secrets still
// reference it so entries never end up in a category that no longer exists.
func (v *Vault) DeleteSecretCategory(id int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	var code string
	if err := v.db.QueryRow(`SELECT code FROM secret_categories WHERE id = ?`, id).Scan(&code); err != nil {
		return err
	}
	var n int
	if err := v.db.QueryRow(`SELECT COUNT(1) FROM secret_entries WHERE category = ?`, code).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrSecretCategoryInUse
	}
	_, err := v.db.Exec(`DELETE FROM secret_categories WHERE id = ?`, id)
	return err
}

// listSecretCategoryRows returns every category with its name decrypted, for backup.
func (v *Vault) listSecretCategoryRows() ([]store.Category, error) {
	return v.ListSecretCategories()
}

// restoreSecretCategoryRows re-inserts categories from a backup, keeping their
// codes so imported secrets still resolve. Codes that already exist are skipped.
func (v *Vault) restoreSecretCategoryRows(cats []store.Category) error {
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
		if err := v.db.QueryRow(`SELECT COUNT(1) FROM secret_categories WHERE code = ?`, cat.Code).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		ct, nonce, err := c.EncryptField(cat.Name, aadSecretCatName)
		if err != nil {
			return err
		}
		order := cat.SortOrder
		if order == 0 {
			order = i
		}
		if _, err := v.db.Exec(`INSERT INTO secret_categories
            (code, name_ct, name_nonce, sort_order, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?)`, cat.Code, ct, nonce, order, now, now); err != nil {
			return err
		}
	}
	return nil
}

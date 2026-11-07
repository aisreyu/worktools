package vault

import (
	"database/sql"
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// ---- M1: URL / credential entries ----

// ListURLs returns all URL entries with passwords decrypted.
func (v *Vault) ListURLs() ([]store.URLEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, title, url, env, tags, username, password_ct, password_nonce, note, created_at, updated_at
        FROM url_entries ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.URLEntry
	for rows.Next() {
		var (
			e             store.URLEntry
			ct, nonce     []byte
		)
		if err := rows.Scan(&e.ID, &e.Title, &e.URL, &e.Env, &e.Tags, &e.Username, &ct, &nonce, &e.Note, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		if e.Password, err = c.DecryptField(ct, nonce, aadURLPass); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveURL inserts (ID==0) or updates a URL entry.
func (v *Vault) SaveURL(e store.URLEntry) (int64, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return 0, err
	}
	ct, nonce, err := c.EncryptField(e.Password, aadURLPass)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	if e.ID == 0 {
		res, err := v.db.Exec(`INSERT INTO url_entries
            (title, url, env, tags, username, password_ct, password_nonce, note, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, e.URL, e.Env, e.Tags, e.Username, ct, nonce, e.Note, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = v.db.Exec(`UPDATE url_entries SET title=?, url=?, env=?, tags=?, username=?,
        password_ct=?, password_nonce=?, note=?, updated_at=? WHERE id=?`,
		e.Title, e.URL, e.Env, e.Tags, e.Username, ct, nonce, e.Note, now, e.ID)
	return e.ID, err
}

// DeleteURL removes a URL entry.
func (v *Vault) DeleteURL(id int64) error {
	_, err := v.db.Exec(`DELETE FROM url_entries WHERE id = ?`, id)
	return err
}

// ---- M2: personal password vault ----

// ListSecrets returns all secret entries with passwords decrypted.
func (v *Vault) ListSecrets() ([]store.SecretEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, title, category, host, port, username, password_ct, password_nonce,
        extra_ct, extra_nonce, note, created_at, updated_at FROM secret_entries ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.SecretEntry
	for rows.Next() {
		var (
			e                     store.SecretEntry
			pct, pn, ect, en      []byte
		)
		if err := rows.Scan(&e.ID, &e.Title, &e.Category, &e.Host, &e.Port, &e.Username, &pct, &pn, &ect, &en, &e.Note, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		if e.Password, err = c.DecryptField(pct, pn, aadSecPass); err != nil {
			return nil, err
		}
		if e.Extra, err = c.DecryptField(ect, en, aadSecExtra); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveSecret inserts (ID==0) or updates a secret entry.
func (v *Vault) SaveSecret(e store.SecretEntry) (int64, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return 0, err
	}
	pct, pn, err := c.EncryptField(e.Password, aadSecPass)
	if err != nil {
		return 0, err
	}
	ect, en, err := c.EncryptField(e.Extra, aadSecExtra)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	if e.ID == 0 {
		res, err := v.db.Exec(`INSERT INTO secret_entries
            (title, category, host, port, username, password_ct, password_nonce, extra_ct, extra_nonce, note, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, e.Category, e.Host, e.Port, e.Username, pct, pn, ect, en, e.Note, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = v.db.Exec(`UPDATE secret_entries SET title=?, category=?, host=?, port=?, username=?,
        password_ct=?, password_nonce=?, extra_ct=?, extra_nonce=?, note=?, updated_at=? WHERE id=?`,
		e.Title, e.Category, e.Host, e.Port, e.Username, pct, pn, ect, en, e.Note, now, e.ID)
	return e.ID, err
}

// DeleteSecret removes a secret entry.
func (v *Vault) DeleteSecret(id int64) error {
	_, err := v.db.Exec(`DELETE FROM secret_entries WHERE id = ?`, id)
	return err
}

// ---- M3: OTP entries ----

// ListOTPs returns all OTP entries with secrets decrypted.
func (v *Vault) ListOTPs() ([]store.OTPEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, title, issuer, account, secret_ct, secret_nonce, algorithm, digits, period, otp_type, counter, created_at, updated_at
        FROM otp_entries ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.OTPEntry
	for rows.Next() {
		var (
			e         store.OTPEntry
			sct, snon []byte
		)
		if err := rows.Scan(&e.ID, &e.Title, &e.Issuer, &e.Account, &sct, &snon, &e.Algorithm, &e.Digits, &e.Period, &e.Type, &e.Counter, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		if e.Secret, err = c.DecryptField(sct, snon, aadOTPSecret); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetOTP returns a single OTP entry by id.
func (v *Vault) GetOTP(id int64) (store.OTPEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return store.OTPEntry{}, err
	}
	var (
		e         store.OTPEntry
		sct, snon []byte
	)
	err = v.db.QueryRow(`SELECT id, title, issuer, account, secret_ct, secret_nonce, algorithm, digits, period, otp_type, counter, created_at, updated_at
        FROM otp_entries WHERE id = ?`, id).
		Scan(&e.ID, &e.Title, &e.Issuer, &e.Account, &sct, &snon, &e.Algorithm, &e.Digits, &e.Period, &e.Type, &e.Counter, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return store.OTPEntry{}, err
	}
	e.Secret, err = c.DecryptField(sct, snon, aadOTPSecret)
	return e, err
}

// SaveOTP inserts (ID==0) or updates an OTP entry.
func (v *Vault) SaveOTP(e store.OTPEntry) (int64, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return 0, err
	}
	sct, snon, err := c.EncryptField(e.Secret, aadOTPSecret)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	if e.ID == 0 {
		res, err := v.db.Exec(`INSERT INTO otp_entries
            (title, issuer, account, secret_ct, secret_nonce, algorithm, digits, period, otp_type, counter, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, e.Issuer, e.Account, sct, snon, e.Algorithm, e.Digits, e.Period, e.Type, e.Counter, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = v.db.Exec(`UPDATE otp_entries SET title=?, issuer=?, account=?, secret_ct=?, secret_nonce=?,
        algorithm=?, digits=?, period=?, otp_type=?, counter=?, updated_at=? WHERE id=?`,
		e.Title, e.Issuer, e.Account, sct, snon, e.Algorithm, e.Digits, e.Period, e.Type, e.Counter, now, e.ID)
	return e.ID, err
}

// DeleteOTP removes an OTP entry.
func (v *Vault) DeleteOTP(id int64) error {
	_, err := v.db.Exec(`DELETE FROM otp_entries WHERE id = ?`, id)
	return err
}

// BumpOTPCounter increments the HOTP counter and returns the new value.
func (v *Vault) BumpOTPCounter(id int64) (int64, error) {
	_, err := v.db.Exec(`UPDATE otp_entries SET counter = counter + 1, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return 0, err
	}
	var n int64
	err = v.db.QueryRow(`SELECT counter FROM otp_entries WHERE id = ?`, id).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return n, nil
}

package vault

import (
	"database/sql"
	"time"
)

// AppSecretKeyAutoBackup is the app_settings key holding the encrypted backup
// password used by the auto-backup scheduler.
const AppSecretKeyAutoBackup = "autobackup.password"

// AppSecret keys for the S3-compatible cloud backup target. The secret key is
// stored under its own key so it can be rotated independently of the AK.
const (
	AppSecretKeyCloudAK = "cloudbackup.access_key"
	AppSecretKeyCloudSK = "cloudbackup.secret_key"
)

// SetAppSecret stores an encrypted application secret under key. The vault
// must be unlocked.
func (v *Vault) SetAppSecret(key, value string) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	ct, nonce, err := c.EncryptField(value, aadAppSecret)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`INSERT INTO app_settings (key, value_ct, value_nonce, updated_at)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(key) DO UPDATE SET value_ct=excluded.value_ct,
            value_nonce=excluded.value_nonce, updated_at=excluded.updated_at`,
		key, ct, nonce, time.Now().Unix())
	return err
}

// GetAppSecret returns the decrypted secret stored under key. It returns
// ErrLocked when the vault is locked and sql.ErrNoRows when the key is absent.
func (v *Vault) GetAppSecret(key string) (string, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return "", err
	}
	var ct, nonce []byte
	err = v.db.QueryRow(`SELECT value_ct, value_nonce FROM app_settings WHERE key = ?`, key).Scan(&ct, &nonce)
	if err == sql.ErrNoRows {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", err
	}
	return c.DecryptField(ct, nonce, aadAppSecret)
}

// HasAppSecret reports whether key is present (without decrypting).
func (v *Vault) HasAppSecret(key string) bool {
	var one int
	err := v.db.QueryRow(`SELECT 1 FROM app_settings WHERE key = ?`, key).Scan(&one)
	return err == nil
}

// DeleteAppSecret removes the secret stored under key.
func (v *Vault) DeleteAppSecret(key string) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	_, err := v.db.Exec(`DELETE FROM app_settings WHERE key = ?`, key)
	return err
}

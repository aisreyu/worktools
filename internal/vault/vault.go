package vault

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

// AAD constants bind ciphertext to its logical column.
const (
	aadWrap        = "vault_meta:wrap"
	aadURLPass     = "url_entries:password"
	aadSecPass     = "secret_entries:password"
	aadSecExtra    = "secret_entries:extra"
	aadOTPSecret   = "otp_entries:secret"
	aadNote        = "note_entries:content"
	aadNoteAttach  = "note_entries:attachment"
	aadRecAnswers  = "recovery_meta:answers"
	aadRecWrap     = "recovery_meta:wrap"
	aadFileData    = "file_entries:data"
	aadFileCatName = "file_categories:name"
	aadClipText    = "clipboard_items:text"
	aadClipSetting = "app_settings:clipboard"
)

// Vault ties the SQLite database to the in-memory key material. It is the only
// component that knows how to turn stored ciphertext into plaintext.
type Vault struct {
	db *sql.DB

	mu     sync.RWMutex
	crypto *Crypto
}

// New wraps an already-open database.
func New(db *sql.DB) *Vault { return &Vault{db: db} }

// Initialised reports whether the vault has been created (master password set).
func (v *Vault) Initialised() (bool, error) {
	var n int
	err := v.db.QueryRow(`SELECT COUNT(1) FROM vault_meta`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Locked reports whether the vault currently has no usable key.
func (v *Vault) Locked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.crypto == nil
}

// Init creates the vault with a fresh master password and a random DEK.
func (v *Vault) Init(password string) error {
	if len(password) < 8 {
		return ErrShortPassword
	}
	ok, err := v.Initialised()
	if err != nil {
		return err
	}
	if ok {
		return ErrAlreadyInit
	}

	salt, err := newRandom(16)
	if err != nil {
		return err
	}
	params := DefaultKDFParams
	masterKey := deriveKey(password, salt, params)
	defer zero(masterKey)

	dek, err := newRandom(32)
	if err != nil {
		return err
	}
	wrapCT, wrapNonce, err := wrapDEK(masterKey, dek, aadWrap)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err = v.db.Exec(`INSERT INTO vault_meta
        (id, version, kdf, kdf_salt, kdf_params, wrap_cipher, wrap_nonce, created_at, updated_at)
        VALUES (1, 1, 'argon2id', ?, ?, ?, ?, ?, ?)`,
		salt, marshalParams(params), wrapCT, wrapNonce, now, now)
	if err != nil {
		return fmt.Errorf("store vault meta: %w", err)
	}

	c, err := newCrypto(dek, params)
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.crypto = c
	v.mu.Unlock()
	if err := v.EnsureCategoryDefaults(); err != nil {
		return err
	}
	return nil
}

// Unlock derives the master key and unwraps the DEK. On a wrong password the
// AEAD open fails and ErrBadPassword is returned.
func (v *Vault) Unlock(password string) error {
	var (
		salt      []byte
		paramsRaw string
		wrapCT    []byte
		wrapNonce []byte
	)
	err := v.db.QueryRow(`SELECT kdf_salt, kdf_params, wrap_cipher, wrap_nonce FROM vault_meta WHERE id = 1`).
		Scan(&salt, &paramsRaw, &wrapCT, &wrapNonce)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotInit
	}
	if err != nil {
		return err
	}
	params, err := unmarshalParams(paramsRaw)
	if err != nil {
		return err
	}

	masterKey := deriveKey(password, salt, params)
	defer zero(masterKey)

	dek, err := unwrapDEK(masterKey, wrapCT, wrapNonce, aadWrap)
	if err != nil {
		return ErrBadPassword
	}
	c, err := newCrypto(dek, params)
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.crypto = c
	v.mu.Unlock()
	if err := v.EnsureCategoryDefaults(); err != nil {
		return err
	}
	return nil
}

// Lock wipes the in-memory key material.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.crypto != nil {
		v.crypto.Zero()
		v.crypto = nil
	}
}

// ChangePassword re-wraps the DEK under a new master password.
func (v *Vault) ChangePassword(oldPwd, newPwd string) error {
	if len(newPwd) < 8 {
		return ErrShortPassword
	}
	if err := v.Unlock(oldPwd); err != nil {
		return err
	}
	salt, err := newRandom(16)
	if err != nil {
		return err
	}
	params := DefaultKDFParams
	masterKey := deriveKey(newPwd, salt, params)
	defer zero(masterKey)

	v.mu.RLock()
	dek := append([]byte(nil), v.crypto.dek...)
	v.mu.RUnlock()
	defer zero(dek)

	wrapCT, wrapNonce, err := wrapDEK(masterKey, dek, aadWrap)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`UPDATE vault_meta SET kdf_salt = ?, kdf_params = ?, wrap_cipher = ?, wrap_nonce = ?, updated_at = ? WHERE id = 1`,
		salt, marshalParams(params), wrapCT, wrapNonce, time.Now().Unix())
	return err
}

// cryptoOrErr returns the active crypto or ErrLocked.
func (v *Vault) cryptoOrErr() (*Crypto, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.crypto == nil {
		return nil, ErrLocked
	}
	return v.crypto, nil
}

// zero overwrites a byte slice.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

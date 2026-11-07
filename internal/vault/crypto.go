// Package vault implements the encrypted "safe" used by notebook.
//
// Design:
//   - The user's master password is never stored. It is turned into a 32-byte
//     master key with Argon2id.
//   - The master key wraps a random Data Encryption Key (DEK) with AES-256-GCM.
//   - Every sensitive field is encrypted with the DEK using AES-256-GCM and a
//     fresh random nonce. The table+column name is bound as Additional
//     Authenticated Data so ciphertext cannot be moved between fields.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// KDFParams describes the Argon2id cost parameters. They are stored alongside
// the vault so a future release can raise the cost without breaking old files.
type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"` // KiB
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

// DefaultKDFParams is a reasonable interactive-login cost.
var DefaultKDFParams = KDFParams{Time: 3, Memory: 64 * 1024, Threads: 4, KeyLen: 32}

// Common errors returned by the vault.
var (
	ErrLocked       = errors.New("vault is locked")
	ErrBadPassword  = errors.New("invalid master password")
	ErrNotInit      = errors.New("vault has not been initialised")
	ErrAlreadyInit  = errors.New("vault is already initialised")
	ErrShortPassword = errors.New("master password must be at least 8 characters")
)

// Crypto holds the key material for an unlocked vault. It lives only in memory
// and is zeroed on Lock.
type Crypto struct {
	dek    []byte // data encryption key
	gcm    cipher.AEAD
	params KDFParams
}

// deriveKey runs Argon2id over the master password and salt.
func deriveKey(password string, salt []byte, p KDFParams) []byte {
	return argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
}

// newRandom returns n cryptographically random bytes.
func newRandom(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// seal encrypts plaintext with the given AAD using a fresh nonce.
func (c *Crypto) seal(plaintext []byte, aad string) (ciphertext, nonce []byte, err error) {
	nonce, err = newRandom(c.gcm.NonceSize())
	if err != nil {
		return nil, nil, err
	}
	ct := c.gcm.Seal(nil, nonce, plaintext, []byte(aad))
	return ct, nonce, nil
}

// open decrypts ciphertext with the given nonce and AAD.
func (c *Crypto) open(ciphertext, nonce []byte, aad string) ([]byte, error) {
	return c.gcm.Open(nil, nonce, ciphertext, []byte(aad))
}

// EncryptField encrypts a string field bound to aad (e.g. "url_entries:password").
func (c *Crypto) EncryptField(plaintext, aad string) (ct, nonce []byte, err error) {
	return c.seal([]byte(plaintext), aad)
}

// DecryptField reverses EncryptField.
func (c *Crypto) DecryptField(ct, nonce []byte, aad string) (string, error) {
	if len(ct) == 0 {
		return "", nil
	}
	pt, err := c.open(ct, nonce, aad)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// Zero wipes the key material.
func (c *Crypto) Zero() {
	if c == nil {
		return
	}
	subtle.ConstantTimeCopy(1, c.dek, make([]byte, len(c.dek)))
	if c.gcm != nil {
		c.gcm = nil
	}
}

// WrapDEK encrypts the DEK with the master key derived from password.
func wrapDEK(masterKey, dek []byte, aad string) (ct, nonce []byte, err error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = newRandom(gcm.NonceSize())
	if err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, dek, []byte(aad)), nonce, nil
}

// unwrapDEK reverses wrapDEK.
func unwrapDEK(masterKey, ct, nonce []byte, aad string) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, []byte(aad))
}

// newCrypto builds a Crypto bound to the given DEK.
func newCrypto(dek []byte, params KDFParams) (*Crypto, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Crypto{dek: append([]byte(nil), dek...), gcm: gcm, params: params}, nil
}

// marshalParams is a helper for storing KDF params as JSON.
func marshalParams(p KDFParams) string {
	b, _ := json.Marshal(p)
	return string(b)
}

// unmarshalParams is a helper for reading KDF params from JSON.
func unmarshalParams(s string) (KDFParams, error) {
	var p KDFParams
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return p, fmt.Errorf("decode kdf params: %w", err)
	}
	return p, nil
}

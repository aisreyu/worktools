package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// Backup format: JSON envelope encrypted with AES-256-GCM under a key derived
// from the master password given at export time. The file is self-contained:
// KDF salt + params + ciphertext, so it can be restored on any machine.
type backupEnvelope struct {
	Format    string    `json:"format"` // "notebook-backup"
	Version   int       `json:"version"`
	KDF       string    `json:"kdf"`
	Salt      string    `json:"salt"` // base64
	Params    KDFParams `json:"params"`
	Nonce     string    `json:"nonce"`  // base64
	Cipher    string    `json:"cipher"` // base64
	CreatedAt int64     `json:"createdAt"`
}

// backupPayload is the plaintext JSON inside the envelope.
type backupPayload struct {
	URLs       []store.URLEntry      `json:"urls"`
	Secrets    []store.SecretEntry   `json:"secrets"`
	OTPs       []store.OTPEntry      `json:"otps"`
	Notes      []store.NoteEntry     `json:"notes"`
	Schedules  []store.ScheduleEntry `json:"schedules"`
	Files      []store.FileEntry     `json:"files"`
	Categories []store.FileCategory  `json:"categories,omitempty"`
}

const backupAAD = "notebook:backup"

// ErrBadBackup is returned when a backup file cannot be decrypted or parsed.
var ErrBadBackup = errors.New("invalid backup file or wrong password")

// ExportBackup encrypts the whole vault content with a key derived from the
// given password and returns the file bytes. The vault must be unlocked.
func (v *Vault) ExportBackup(password string) ([]byte, error) {
	if _, err := v.cryptoOrErr(); err != nil {
		return nil, err
	}
	urls, err := v.ListURLs()
	if err != nil {
		return nil, err
	}
	secrets, err := v.ListSecrets()
	if err != nil {
		return nil, err
	}
	otps, err := v.ListOTPs()
	if err != nil {
		return nil, err
	}
	notes, err := v.ListNotes()
	if err != nil {
		return nil, err
	}
	schedules, err := v.ListSchedules()
	if err != nil {
		return nil, err
	}
	files, err := v.listFileRows()
	if err != nil {
		return nil, err
	}
	categories, err := v.listCategoryRows()
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(backupPayload{URLs: urls, Secrets: secrets, OTPs: otps, Notes: notes, Schedules: schedules, Files: files, Categories: categories})
	if err != nil {
		return nil, err
	}

	salt, err := newRandom(16)
	if err != nil {
		return nil, err
	}
	params := DefaultKDFParams
	key := deriveKey(password, salt, params)
	defer zero(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := newRandom(gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, payload, []byte(backupAAD))

	env := backupEnvelope{
		Format:    "notebook-backup",
		Version:   1,
		KDF:       "argon2id",
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Params:    params,
		Nonce:     base64.StdEncoding.EncodeToString(nonce),
		Cipher:    base64.StdEncoding.EncodeToString(ct),
		CreatedAt: time.Now().Unix(),
	}
	return json.MarshalIndent(env, "", "  ")
}

// ImportBackup decrypts a backup file and merges its entries into the vault.
// The vault must be unlocked. Existing entries are kept; imported entries are
// appended as new rows.
func (v *Vault) ImportBackup(data []byte, password string) (imported int, err error) {
	if _, err := v.cryptoOrErr(); err != nil {
		return 0, err
	}
	var env backupEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return 0, ErrBadBackup
	}
	if env.Format != "notebook-backup" {
		return 0, ErrBadBackup
	}
	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return 0, ErrBadBackup
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return 0, ErrBadBackup
	}
	ct, err := base64.StdEncoding.DecodeString(env.Cipher)
	if err != nil {
		return 0, ErrBadBackup
	}

	key := deriveKey(password, salt, env.Params)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, ErrBadBackup
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, ErrBadBackup
	}
	payload, err := gcm.Open(nil, nonce, ct, []byte(backupAAD))
	if err != nil {
		return 0, ErrBadBackup
	}

	var p backupPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return 0, ErrBadBackup
	}

	n := 0
	for _, e := range p.URLs {
		e.ID = 0
		if _, err := v.SaveURL(e); err != nil {
			return n, err
		}
		n++
	}
	for _, e := range p.Secrets {
		e.ID = 0
		if _, err := v.SaveSecret(e); err != nil {
			return n, err
		}
		n++
	}
	for _, e := range p.OTPs {
		e.ID = 0
		if _, err := v.SaveOTP(e); err != nil {
			return n, err
		}
		n++
	}
	for _, e := range p.Notes {
		e.ID = 0
		if _, err := v.SaveNote(e); err != nil {
			return n, err
		}
		n++
	}
	for _, e := range p.Schedules {
		e.ID = 0
		if _, err := v.SaveSchedule(e); err != nil {
			return n, err
		}
		n++
	}
	for _, e := range p.Files {
		e.ID = 0
		if e.Data == "" {
			continue
		}
		if _, err := v.SaveFile(e); err != nil {
			return n, err
		}
		n++
	}
	if err := v.restoreCategoryRows(p.Categories); err != nil {
		return n, err
	}
	return n, nil
}

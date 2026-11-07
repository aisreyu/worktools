package vault

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
)

// Recovery errors.
var (
	ErrNoRecovery     = errors.New("no recovery questions configured")
	ErrBadRecovery    = errors.New("recovery answers do not match")
	ErrNeedQuestions  = errors.New("at least one question with answer is required")
)

// recoveryRow is the on-disk shape of recovery_meta.
type recoveryRow struct {
	Questions      []string
	AnswersCT      []byte
	AnswersNonce   []byte
	RecSalt        []byte
	RecParams      string
	RecWrapCT      []byte
	RecWrapNonce   []byte
}

// HasRecovery reports whether recovery questions are configured.
func (v *Vault) HasRecovery() (bool, error) {
	var n int
	err := v.db.QueryRow(`SELECT COUNT(1) FROM recovery_meta`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RecoveryQuestions returns the configured question texts (no answers).
// Works while locked so the unlock screen can show them.
func (v *Vault) RecoveryQuestions() ([]string, error) {
	var raw string
	err := v.db.QueryRow(`SELECT questions FROM recovery_meta WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoRecovery
	}
	if err != nil {
		return nil, err
	}
	var qs []string
	if err := json.Unmarshal([]byte(raw), &qs); err != nil {
		return nil, fmt.Errorf("decode questions: %w", err)
	}
	return qs, nil
}

// SetRecoveryQuestions stores the questions and answers, and wraps a copy of
// the DEK under a key derived from the answers. Requires an unlocked vault.
func (v *Vault) SetRecoveryQuestions(qs []store.RecoveryQuestion) error {
	c, err := v.cryptoOrErr()
	if err != nil {
		return err
	}
	if len(qs) == 0 {
		return ErrNeedQuestions
	}
	questions := make([]string, 0, len(qs))
	answers := make([]string, 0, len(qs))
	for _, q := range qs {
		qq := strings.TrimSpace(q.Question)
		aa := strings.TrimSpace(q.Answer)
		if qq == "" || aa == "" {
			return ErrNeedQuestions
		}
		questions = append(questions, qq)
		answers = append(answers, strings.ToLower(aa))
	}

	qJSON, _ := json.Marshal(questions)
	aJSON, _ := json.Marshal(answers)
	aCT, aNonce, err := c.EncryptField(string(aJSON), aadRecAnswers)
	if err != nil {
		return err
	}

	// Recovery key = Argon2id(normalised answers joined, salt).
	salt, err := newRandom(16)
	if err != nil {
		return err
	}
	params := DefaultKDFParams
	recKey := deriveKey(strings.Join(answers, "\x1f"), salt, params)
	defer zero(recKey)

	v.mu.RLock()
	dek := append([]byte(nil), c.dek...)
	v.mu.RUnlock()
	defer zero(dek)

	wrapCT, wrapNonce, err := wrapDEK(recKey, dek, aadRecWrap)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err = v.db.Exec(`INSERT INTO recovery_meta
        (id, questions, answers_ct, answers_nonce, rec_salt, rec_params, rec_wrap_cipher, rec_wrap_nonce, created_at, updated_at)
        VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET questions=excluded.questions, answers_ct=excluded.answers_ct,
            answers_nonce=excluded.answers_nonce, rec_salt=excluded.rec_salt, rec_params=excluded.rec_params,
            rec_wrap_cipher=excluded.rec_wrap_cipher, rec_wrap_nonce=excluded.rec_wrap_nonce, updated_at=excluded.updated_at`,
		string(qJSON), aCT, aNonce, salt, marshalParams(params), wrapCT, wrapNonce, now, now)
	return err
}

// loadRecovery reads the recovery row.
func (v *Vault) loadRecovery() (*recoveryRow, error) {
	var (
		r        recoveryRow
		qRaw     string
	)
	err := v.db.QueryRow(`SELECT questions, answers_ct, answers_nonce, rec_salt, rec_params, rec_wrap_cipher, rec_wrap_nonce
        FROM recovery_meta WHERE id = 1`).
		Scan(&qRaw, &r.AnswersCT, &r.AnswersNonce, &r.RecSalt, &r.RecParams, &r.RecWrapCT, &r.RecWrapNonce)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoRecovery
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(qRaw), &r.Questions); err != nil {
		return nil, fmt.Errorf("decode questions: %w", err)
	}
	return &r, nil
}

// ResetPasswordWithRecovery verifies the answers against the stored encrypted
// answers, then re-wraps the DEK under the new master password. Works while
// locked. On success the vault stays locked; the user unlocks with the new
// password.
func (v *Vault) ResetPasswordWithRecovery(answers []string, newPwd string) error {
	if len(newPwd) < 8 {
		return ErrShortPassword
	}
	r, err := v.loadRecovery()
	if err != nil {
		return err
	}

	// Normalise the supplied answers the same way as when stored.
	norm := make([]string, 0, len(answers))
	for _, a := range answers {
		norm = append(norm, strings.ToLower(strings.TrimSpace(a)))
	}

	// Verify against the encrypted answers using the active crypto if
	// unlocked; otherwise derive the recovery key and unwrap the DEK copy.
	var dek []byte
	if !v.Locked() {
		c, err := v.cryptoOrErr()
		if err != nil {
			return err
		}
		aJSON, err := c.DecryptField(r.AnswersCT, r.AnswersNonce, aadRecAnswers)
		if err != nil {
			return err
		}
		var stored []string
		if err := json.Unmarshal([]byte(aJSON), &stored); err != nil {
			return fmt.Errorf("decode answers: %w", err)
		}
		if !answersEqual(stored, norm) {
			return ErrBadRecovery
		}
		v.mu.RLock()
		dek = append([]byte(nil), c.dek...)
		v.mu.RUnlock()
	} else {
		// Locked: derive the recovery key from the supplied answers and try to
		// unwrap the recovery DEK copy. A wrong answer produces a wrong key and
		// the AEAD open fails, which we report as ErrBadRecovery.
		params, err := unmarshalParams(r.RecParams)
		if err != nil {
			return err
		}
		recKey := deriveKey(strings.Join(norm, "\x1f"), r.RecSalt, params)
		defer zero(recKey)
		dek, err = unwrapDEK(recKey, r.RecWrapCT, r.RecWrapNonce, aadRecWrap)
		if err != nil {
			return ErrBadRecovery
		}
	}
	defer zero(dek)

	// Re-wrap the DEK under the new master password.
	salt, err := newRandom(16)
	if err != nil {
		return err
	}
	params := DefaultKDFParams
	masterKey := deriveKey(newPwd, salt, params)
	defer zero(masterKey)

	wrapCT, wrapNonce, err := wrapDEK(masterKey, dek, aadWrap)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`UPDATE vault_meta SET kdf_salt = ?, kdf_params = ?, wrap_cipher = ?, wrap_nonce = ?, updated_at = ? WHERE id = 1`,
		salt, marshalParams(params), wrapCT, wrapNonce, time.Now().Unix())
	return err
}

// answersEqual compares two answer lists in order.
func answersEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

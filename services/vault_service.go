// Package services exposes the notebook domain to the frontend. Every method
// here becomes a typed binding callable from the web UI. These are thin,
// validating wrappers around the vault, otp and toolbox packages.
package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// VaultService manages locking, unlocking and entry CRUD.
type VaultService struct {
	v        *vault.Vault
	lastUsed time.Time
}

// NewVaultService wires the service to a vault.
func NewVaultService(v *vault.Vault) *VaultService {
	return &VaultService{v: v, lastUsed: time.Now()}
}

func (s *VaultService) touch() { s.lastUsed = time.Now() }

// Status reports whether the vault exists and is unlocked.
type Status struct {
	Initialised bool `json:"initialised"`
	Unlocked    bool `json:"unlocked"`
}

// Status returns the current vault state.
func (s *VaultService) Status() Status {
	ok, _ := s.v.Initialised()
	return Status{Initialised: ok, Unlocked: !s.v.Locked()}
}

// Init creates a new vault with the given master password.
func (s *VaultService) Init(password string) error {
	if err := s.v.Init(password); err != nil {
		return err
	}
	s.touch()
	return nil
}

// Unlock opens the vault with the master password.
func (s *VaultService) Unlock(password string) error {
	if err := s.v.Unlock(password); err != nil {
		return err
	}
	s.touch()
	// Enforce the trash retention window on each unlock (best-effort, fast).
	_, _ = s.v.PurgeTrashOlderThan(30 * 24 * 3600)
	return nil
}

// Lock wipes the in-memory keys.
func (s *VaultService) Lock() {
	s.v.Lock()
}

// ChangePassword rewrites the master password.
func (s *VaultService) ChangePassword(oldPwd, newPwd string) error {
	return s.v.ChangePassword(oldPwd, newPwd)
}

// ---- Recovery questions ----

// HasRecovery reports whether security questions are configured.
func (s *VaultService) HasRecovery() (bool, error) {
	return s.v.HasRecovery()
}

// RecoveryQuestions returns the stored question texts (works while locked).
func (s *VaultService) RecoveryQuestions() ([]string, error) {
	return s.v.RecoveryQuestions()
}

// SetRecoveryQuestions stores questions and answers (unlocked only).
func (s *VaultService) SetRecoveryQuestions(qs []store.RecoveryQuestion) error {
	return s.v.SetRecoveryQuestions(qs)
}

// ResetPasswordWithRecovery resets the master password after verifying the
// answers. Works while locked.
func (s *VaultService) ResetPasswordWithRecovery(answers []string, newPwd string) error {
	return s.v.ResetPasswordWithRecovery(answers, newPwd)
}

// IdleSeconds returns seconds since the last mutating call.
func (s *VaultService) IdleSeconds() int {
	return int(time.Since(s.lastUsed).Seconds())
}

// ---- M1: URLs ----

// ListURLs returns all URL entries.
func (s *VaultService) ListURLs() ([]store.URLEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListURLs()
}

// SaveURL creates or updates a URL entry.
func (s *VaultService) SaveURL(e store.URLEntry) (int64, error) {
	if strings.TrimSpace(e.Title) == "" && strings.TrimSpace(e.URL) == "" {
		return 0, fmt.Errorf("title or url is required")
	}
	s.touch()
	return s.v.SaveURL(e)
}

// DeleteURL removes a URL entry.
func (s *VaultService) DeleteURL(id int64) error {
	s.touch()
	return s.v.DeleteURL(id)
}

// SetURLPinned pins or unpins a URL entry.
func (s *VaultService) SetURLPinned(id int64, pinned bool) error {
	s.touch()
	return s.v.SetURLPinned(id, pinned)
}

// ---- M2: secrets ----

// ListSecrets returns all secret entries.
func (s *VaultService) ListSecrets() ([]store.SecretEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListSecrets()
}

// SaveSecret creates or updates a secret entry.
func (s *VaultService) SaveSecret(e store.SecretEntry) (int64, error) {
	if strings.TrimSpace(e.Title) == "" {
		return 0, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(e.Category) == "" {
		e.Category = "other"
	}
	s.touch()
	return s.v.SaveSecret(e)
}

// DeleteSecret removes a secret entry.
func (s *VaultService) DeleteSecret(id int64) error {
	s.touch()
	return s.v.DeleteSecret(id)
}

// SetSecretPinned pins or unpins a secret entry.
func (s *VaultService) SetSecretPinned(id int64, pinned bool) error {
	s.touch()
	return s.v.SetSecretPinned(id, pinned)
}

// ---- custom secret categories ----

// ListCategories returns the user-managed secret categories.
func (s *VaultService) ListCategories() ([]store.Category, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListSecretCategories()
}

// CreateCategory adds a user-defined secret category and returns it.
func (s *VaultService) CreateCategory(name string) (store.Category, error) {
	if s.v.Locked() {
		return store.Category{}, vault.ErrLocked
	}
	s.touch()
	return s.v.CreateSecretCategory(name)
}

// RenameCategory changes a secret category's display name.
func (s *VaultService) RenameCategory(id int64, name string) error {
	s.touch()
	return s.v.RenameSecretCategory(id, name)
}

// DeleteCategory removes a secret category; it errors when secrets still use it.
func (s *VaultService) DeleteCategory(id int64) error {
	s.touch()
	return s.v.DeleteSecretCategory(id)
}

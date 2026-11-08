package services

import (
	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// TrashService exposes the recycle bin: list, restore, purge and empty. It also
// runs a retention purge so old trash is cleared automatically.
type TrashService struct {
	v *vault.Vault
}

// NewTrashService wires the service to a vault.
func NewTrashService(v *vault.Vault) *TrashService { return &TrashService{v: v} }

// List returns every soft-deleted entry across all modules.
func (s *TrashService) List() ([]store.TrashItem, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListTrash()
}

// Restore puts a trashed entry back into its normal list.
func (s *TrashService) Restore(kind string, id int64) error {
	return s.v.Restore(kind, id)
}

// Purge permanently deletes one trashed entry.
func (s *TrashService) Purge(kind string, id int64) error {
	return s.v.Purge(kind, id)
}

// Empty permanently deletes every trashed entry.
func (s *TrashService) Empty() error {
	return s.v.EmptyTrash()
}

// PurgeOlderThan clears trash deleted more than the given number of days ago
// and returns how many entries were removed. Used at startup to enforce the
// retention window.
func (s *TrashService) PurgeOlderThan(days int) (int64, error) {
	if days <= 0 {
		days = 30
	}
	return s.v.PurgeTrashOlderThan(int64(days) * 24 * 3600)
}

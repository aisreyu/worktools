package services

import (
	"os"
	"strings"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/vault"
)

// BackupService exports and imports encrypted backup files.
type BackupService struct {
	v *vault.Vault
}

// NewBackupService wires the service to a vault.
func NewBackupService(v *vault.Vault) *BackupService { return &BackupService{v: v} }

// Export writes an encrypted backup to path.
func (s *BackupService) Export(path, password string) error {
	if strings.TrimSpace(path) == "" {
		return errText("path is required")
	}
	data, err := s.v.ExportBackup(password)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Import reads an encrypted backup from path and merges its entries.
func (s *BackupService) Import(path, password string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, errText("path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return s.v.ImportBackup(data, password)
}

package services

import (
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// FileService manages the encrypted private folder.
type FileService struct {
	v *vault.Vault
}

// NewFileService wires the service to a vault.
func NewFileService(v *vault.Vault) *FileService { return &FileService{v: v} }

// List returns file metadata (no bodies).
func (s *FileService) List() ([]store.FileEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListFiles()
}

// Get returns a single file with its body decrypted.
func (s *FileService) Get(id int64) (store.FileEntry, error) {
	if s.v.Locked() {
		return store.FileEntry{}, vault.ErrLocked
	}
	return s.v.GetFile(id)
}

// SaveMeta creates or updates metadata. When e.Data is empty on an update the
// stored body is preserved, so category/title/note can be edited alone.
func (s *FileService) SaveMeta(e store.FileEntry) (int64, error) {
	if strings.TrimSpace(e.Title) == "" {
		return 0, errText("title is required")
	}
	if e.Category == "" {
		e.Category = "other"
	}
	return s.v.SaveFile(e)
}

// SaveFromFile encrypts a file picked from disk into a new entry.
func (s *FileService) SaveFromFile(path, title, category, note string) (int64, error) {
	if s.v.Locked() {
		return 0, vault.ErrLocked
	}
	if strings.TrimSpace(path) == "" {
		return 0, errText("path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, errText("read file: " + err.Error())
	}
	if strings.TrimSpace(title) == "" {
		title = filepath.Base(path)
	}
	if category == "" {
		category = "other"
	}
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return s.v.SaveFile(store.FileEntry{
		Title:    title,
		Category: category,
		Note:     note,
		Mime:     mimeType,
		Size:     int64(len(b)),
		Data:     base64.StdEncoding.EncodeToString(b),
	})
}

// ReadExternal reads a plain file from disk as base64 (used by the compressor
// to load a source image without going through the vault).
func (s *FileService) ReadExternal(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errText("path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", errText("read file: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Export writes a stored file to path in plaintext, decrypting on the fly.
func (s *FileService) Export(id int64, path string) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	if strings.TrimSpace(path) == "" {
		return errText("path is required")
	}
	e, err := s.v.GetFile(id)
	if err != nil {
		return err
	}
	b, err := base64.StdEncoding.DecodeString(e.Data)
	if err != nil {
		return errText("decode file: " + err.Error())
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errText("create dir: " + err.Error())
	}
	return os.WriteFile(path, b, 0o600)
}

// Delete removes a file entry.
func (s *FileService) Delete(id int64) error {
	return s.v.DeleteFile(id)
}

// ListCategories returns the private-folder categories for the given vault.
func (s *FileService) ListCategories() ([]store.FileCategory, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListCategories()
}

// CreateCategory adds a user-defined category and returns it (id + code).
func (s *FileService) CreateCategory(name string) (store.FileCategory, error) {
	if s.v.Locked() {
		return store.FileCategory{}, vault.ErrLocked
	}
	return s.v.CreateCategory(name)
}

// RenameCategory changes a category's display name without touching its files.
func (s *FileService) RenameCategory(id int64, name string) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	return s.v.RenameCategory(id, name)
}

// DeleteCategory removes a category; it errors when files still reference it.
func (s *FileService) DeleteCategory(id int64) error {
	if s.v.Locked() {
		return vault.ErrLocked
	}
	return s.v.DeleteCategory(id)
}

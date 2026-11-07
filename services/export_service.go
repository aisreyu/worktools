package services

import (
	"os"
	"path/filepath"
	"strings"
)

// ExportService writes exported files (markdown/html) to user-chosen paths.
type ExportService struct{}

// NewExportService creates the service.
func NewExportService() *ExportService { return &ExportService{} }

// SaveText writes UTF-8 text content to path, creating parent dirs as needed.
func (s *ExportService) SaveText(path, content string) error {
	if strings.TrimSpace(path) == "" {
		return errText("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errText("create dir: " + err.Error())
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadText reads a UTF-8 text file (used for future import features).
func (s *ExportService) ReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", errText("read file: " + err.Error())
	}
	return string(b), nil
}

// SaveBinary writes raw bytes (e.g. a PNG QR code) to path. Wails marshals
// []byte arguments from JS as base64 strings, so accept both forms.
func (s *ExportService) SaveBinary(path string, data []byte) error {
	if strings.TrimSpace(path) == "" {
		return errText("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errText("create dir: " + err.Error())
	}
	return os.WriteFile(path, data, 0o644)
}

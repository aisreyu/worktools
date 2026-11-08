package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisreyu/worktools/internal/config"
)

// AppVersion is the application version shown in Settings / About.
const AppVersion = "1.3.0"

// AppService exposes app-level helpers (platform info, window controls).
type AppService struct{}

// NewAppService creates the app service.
func NewAppService() *AppService { return &AppService{} }

// Platform returns the current OS ("darwin", "windows", "linux").
func (s *AppService) Platform() string { return config.Platform() }

// SandboxPath returns a writable path inside the app's private data directory.
// Android has no save-file dialog, so mobile clients write exports here first
// and then hand the file to the OS share/Downloads provider.
func (s *AppService) SandboxPath(name string) (string, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "", errors.New("file name is required")
	}
	dir, err := config.AppDir()
	if err != nil {
		return "", err
	}
	exports := filepath.Join(dir, "exports")
	if err := os.MkdirAll(exports, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(exports, base), nil
}

// Version returns the application version.
func (s *AppService) Version() string { return AppVersion }

// AppInfo describes the app for the About card.
type AppInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Description string `json:"description"`
	Website     string `json:"website"`
}

// Info returns app metadata for the About card.
func (s *AppService) Info() AppInfo {
	return AppInfo{
		Name:        "WorkTools",
		Version:     AppVersion,
		Platform:    config.Platform(),
		Description: "Local-first encrypted ops workbench: URLs & credentials, OTP, notes, calendar & schedules, toolbox.",
		Website:     "https://github.com/aisreyu/worktools",
	}
}

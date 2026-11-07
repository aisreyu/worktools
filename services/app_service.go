package services

import (
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/config"
)

// AppVersion is the application version shown in Settings / About.
const AppVersion = "1.3.0"

// AppService exposes app-level helpers (platform info, window controls).
type AppService struct{}

// NewAppService creates the app service.
func NewAppService() *AppService { return &AppService{} }

// Platform returns the current OS ("darwin", "windows", "linux").
func (s *AppService) Platform() string { return config.Platform() }

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
		Website:     "https://gitlab.tongdun.cn/anxiang.peng/notebook",
	}
}

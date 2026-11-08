// Package settings stores non-secret application preferences (such as the
// auto-backup schedule) as a plain JSON file. Secrets — like the backup
// password — are never written here; they live encrypted in the vault.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aisreyu/worktools/internal/config"
)

// AutoBackup holds the auto-backup preferences. The backup password itself is
// stored encrypted inside the vault, not here.
type AutoBackup struct {
	Enabled       bool   `json:"enabled"`
	Dir           string `json:"dir"`
	IntervalHours int    `json:"intervalHours"`
	Keep          int    `json:"keep"`
	LastAt        int64  `json:"lastAt"` // unix seconds of the last successful run
}

// Settings is the on-disk preference document.
type Settings struct {
	AutoBackup AutoBackup `json:"autoBackup"`
}

// DefaultAutoBackup returns sensible defaults: a backups folder inside the app
// directory, once a day, keeping the most recent week.
func DefaultAutoBackup() AutoBackup {
	dir := ""
	if appDir, err := config.AppDir(); err == nil {
		dir = filepath.Join(appDir, "backups")
	}
	return AutoBackup{Enabled: false, Dir: dir, IntervalHours: 24, Keep: 7}
}

// Path returns the settings file path (config.AppDir()/settings.json).
func Path() (string, error) {
	dir, err := config.AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// Load reads the settings file, filling in defaults when it is missing or
// partially specified.
func Load() (*Settings, error) {
	s := &Settings{AutoBackup: DefaultAutoBackup()}
	path, err := Path()
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		// Corrupt file: fall back to defaults rather than failing the app.
		return &Settings{AutoBackup: DefaultAutoBackup()}, nil
	}
	def := DefaultAutoBackup()
	if s.AutoBackup.Dir == "" {
		s.AutoBackup.Dir = def.Dir
	}
	if s.AutoBackup.IntervalHours <= 0 {
		s.AutoBackup.IntervalHours = def.IntervalHours
	}
	if s.AutoBackup.Keep <= 0 {
		s.AutoBackup.Keep = def.Keep
	}
	return s, nil
}

// Save atomically writes the settings document to disk.
func Save(s *Settings) error {
	path, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

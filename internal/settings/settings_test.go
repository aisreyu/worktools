package settings

import (
	"os"
	"testing"
)

// The settings file lives under config.AppDir(), which derives from the OS
// config directory. Point the whole lookup at a temp dir so the test never
// touches the real user config.
func withTempAppDir(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	// On macOS os.UserConfigDir uses $HOME/Library/Application Support; setting
	// HOME above is enough. Clear any leftover file just in case.
	if p, err := Path(); err == nil {
		_ = os.Remove(p)
	}
}

func TestLoadDefaultsWhenMissing(t *testing.T) {
	withTempAppDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.AutoBackup.Enabled {
		t.Fatal("auto backup should default to disabled")
	}
	if s.AutoBackup.IntervalHours != 24 {
		t.Fatalf("interval default: got %d want 24", s.AutoBackup.IntervalHours)
	}
	if s.AutoBackup.Keep != 7 {
		t.Fatalf("keep default: got %d want 7", s.AutoBackup.Keep)
	}
	if s.AutoBackup.Dir == "" {
		t.Fatal("default dir should be non-empty")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempAppDir(t)

	s, _ := Load()
	s.AutoBackup = AutoBackup{
		Enabled:       true,
		Dir:           "/tmp/worktools-test-backups",
		IntervalHours: 6,
		Keep:          3,
		LastAt:        1234567890,
	}
	if err := Save(s); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.AutoBackup != s.AutoBackup {
		t.Fatalf("round trip mismatch: got %+v want %+v", got.AutoBackup, s.AutoBackup)
	}
}

func TestLoadToleratesCorruption(t *testing.T) {
	withTempAppDir(t)

	p, err := Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(dirOf(p), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("load corrupt: %v", err)
	}
	if s.AutoBackup.IntervalHours != 24 || s.AutoBackup.Keep != 7 {
		t.Fatalf("corrupt load should fall back to defaults, got %+v", s.AutoBackup)
	}
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}

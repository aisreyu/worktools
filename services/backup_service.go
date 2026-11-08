package services

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aisreyu/worktools/internal/settings"
	"github.com/aisreyu/worktools/internal/vault"
)

// BackupService exports and imports encrypted backup files, and optionally
// keeps a rolling set of local backups on a fixed schedule.
type BackupService struct {
	v *vault.Vault

	mu      sync.Mutex
	runMu   sync.Mutex
	stopCh  chan struct{}
	running bool
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

// BackupFile describes one auto-backup file on disk, for the restore history UI.
type BackupFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// ListBackups returns the auto-backup files in the configured directory, newest
// first. It works whether or not auto-backup is enabled, as long as a dir is set.
func (s *BackupService) ListBackups() ([]BackupFile, error) {
	cfg, err := settings.Load()
	if err != nil {
		return nil, err
	}
	paths := listBackups(cfg.AutoBackup.Dir)
	out := make([]BackupFile, 0, len(paths))
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		out = append(out, BackupFile{
			Name:    filepath.Base(p),
			Path:    p,
			Size:    fi.Size(),
			ModTime: fi.ModTime().Unix(),
		})
	}
	return out, nil
}

// RestoreBackupFile merges a specific auto-backup file into the live vault,
// using the given password. It returns the number of entries imported.
func (s *BackupService) RestoreBackupFile(path, password string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, errText("path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return s.v.ImportBackup(data, password)
}

// ---- 自动备份 ----

// AutoBackupStatus is the UI-facing snapshot of the auto-backup feature.
type AutoBackupStatus struct {
	Enabled       bool   `json:"enabled"`
	Dir           string `json:"dir"`
	IntervalHours int    `json:"intervalHours"`
	Keep          int    `json:"keep"`
	LastAt        int64  `json:"lastAt"`
	NextAt        int64  `json:"nextAt"`
	HasPassword   bool   `json:"hasPassword"`
	Count         int    `json:"count"`
}

// AutoBackupStatus returns the current auto-backup configuration and derived
// schedule times.
func (s *BackupService) AutoBackupStatus() (AutoBackupStatus, error) {
	cfg, err := settings.Load()
	if err != nil {
		return AutoBackupStatus{}, err
	}
	ab := cfg.AutoBackup
	st := AutoBackupStatus{
		Enabled:       ab.Enabled,
		Dir:           ab.Dir,
		IntervalHours: ab.IntervalHours,
		Keep:          ab.Keep,
		LastAt:        ab.LastAt,
		HasPassword:   s.v.HasAppSecret(vault.AppSecretKeyAutoBackup),
	}
	if ab.Enabled && ab.IntervalHours > 0 {
		base := ab.LastAt
		if base == 0 {
			base = time.Now().Unix()
		}
		st.NextAt = base + int64(ab.IntervalHours)*3600
	}
	st.Count = countBackups(ab.Dir)
	return st, nil
}

// SaveAutoBackupSettings persists the schedule (enabled, dir, interval, keep).
// It does not touch the stored password; call SetAutoBackupPassword for that.
func (s *BackupService) SaveAutoBackupSettings(enabled bool, dir string, intervalHours, keep int) error {
	if intervalHours <= 0 {
		intervalHours = 24
	}
	if keep <= 0 {
		keep = 7
	}
	dir = strings.TrimSpace(dir)
	if enabled && dir == "" {
		return errText("备份目录不能为空")
	}
	cfg, err := settings.Load()
	if err != nil {
		return err
	}
	cfg.AutoBackup.Enabled = enabled
	cfg.AutoBackup.Dir = dir
	cfg.AutoBackup.IntervalHours = intervalHours
	cfg.AutoBackup.Keep = keep
	return settings.Save(cfg)
}

// SetAutoBackupPassword stores the encryption password used by scheduled
// backups. The vault must be unlocked.
func (s *BackupService) SetAutoBackupPassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errText("备份密码不能为空")
	}
	return s.v.SetAppSecret(vault.AppSecretKeyAutoBackup, password)
}

// ClearAutoBackupPassword forgets the stored backup password.
func (s *BackupService) ClearAutoBackupPassword() error {
	return s.v.DeleteAppSecret(vault.AppSecretKeyAutoBackup)
}

// RunAutoBackupNow performs one backup immediately using the stored password.
// Returns the path of the file written.
func (s *BackupService) RunAutoBackupNow() (string, error) {
	cfg, err := settings.Load()
	if err != nil {
		return "", err
	}
	return s.runBackup(cfg, time.Now())
}

// runBackup writes one encrypted backup and records the run time, pruning old
// files to the configured keep count. The mutex serialises concurrent calls
// (a manual "back up now" racing the scheduler tick) so two runs in the same
// second cannot clobber each other's file or interleave settings writes.
func (s *BackupService) runBackup(cfg *settings.Settings, now time.Time) (string, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	ab := cfg.AutoBackup
	if strings.TrimSpace(ab.Dir) == "" {
		return "", errText("未配置备份目录")
	}
	password, err := s.v.GetAppSecret(vault.AppSecretKeyAutoBackup)
	if err != nil {
		if err == vault.ErrLocked {
			return "", err
		}
		return "", errText("未设置备份密码，请先在设置中配置")
	}
	if err := os.MkdirAll(ab.Dir, 0o700); err != nil {
		return "", errText("创建备份目录失败: " + err.Error())
	}
	data, err := s.v.ExportBackup(password)
	if err != nil {
		return "", err
	}
	name := backupPrefix + now.Format("20060102-150405") + ".json"
	path := filepath.Join(ab.Dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	pruneBackups(ab.Dir, ab.Keep)
	cfg.AutoBackup.LastAt = now.Unix()
	_ = settings.Save(cfg)
	return path, nil
}

// ---- rolling schedule ----

// StartAutoBackup launches the background scheduler. It is safe to call once
// at startup; the goroutine re-reads settings each tick so later changes take
// effect without a restart.
func (s *BackupService) StartAutoBackup() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	stop := s.stopCh
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}

// StopAutoBackup terminates the scheduler goroutine.
func (s *BackupService) StopAutoBackup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running && s.stopCh != nil {
		close(s.stopCh)
		s.running = false
	}
}

// tick checks whether a scheduled backup is due and, if the vault is unlocked,
// performs it.
func (s *BackupService) tick() {
	cfg, err := settings.Load()
	if err != nil || !cfg.AutoBackup.Enabled {
		return
	}
	if s.v.Locked() {
		return
	}
	interval := int64(cfg.AutoBackup.IntervalHours) * 3600
	if interval <= 0 {
		return
	}
	due := cfg.AutoBackup.LastAt == 0 || time.Now().Unix() >= cfg.AutoBackup.LastAt+interval
	if !due {
		return
	}
	_, _ = s.runBackup(cfg, time.Now())
}

// backupPrefix matches the auto-backup filenames in the target directory.
const backupPrefix = "worktools-autobackup-"

// countBackups counts auto-backup files in dir.
func countBackups(dir string) int {
	return len(listBackups(dir))
}

// listBackups returns the full paths of auto-backup files, newest first.
func listBackups(dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, backupPrefix) && strings.HasSuffix(n, ".json") {
			names = append(names, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = filepath.Join(dir, n)
	}
	return out
}

// pruneBackups keeps the newest keep files and removes the rest.
func pruneBackups(dir string, keep int) {
	if keep <= 0 {
		keep = 7
	}
	files := listBackups(dir)
	for i := keep; i < len(files); i++ {
		_ = os.Remove(files[i])
	}
}

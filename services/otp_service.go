package services

import (
	"os"
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/otp"
	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// OTPService manages OTP entries and generates live codes.
type OTPService struct {
	v *vault.Vault
}

// NewOTPService wires the service to a vault.
func NewOTPService(v *vault.Vault) *OTPService { return &OTPService{v: v} }

// List returns all OTP entries with the secret stripped. Secrets stay in the
// Go process; the UI obtains codes through Code/NextCode.
func (s *OTPService) List() ([]store.OTPEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	entries, err := s.v.ListOTPs()
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Secret = ""
	}
	return entries, nil
}

// Get returns a single entry including its secret, for the edit form.
func (s *OTPService) Get(id int64) (store.OTPEntry, error) {
	if s.v.Locked() {
		return store.OTPEntry{}, vault.ErrLocked
	}
	return s.v.GetOTP(id)
}

// Save creates or updates an OTP entry, normalising defaults.
func (s *OTPService) Save(e store.OTPEntry) (int64, error) {
	if strings.TrimSpace(e.Secret) == "" {
		return 0, errText("secret is required")
	}
	if strings.EqualFold(e.Type, "hotp") {
		e.Type = "hotp"
	} else {
		e.Type = "totp"
	}
	if e.Algorithm == "" {
		e.Algorithm = "SHA1"
	}
	if e.Digits == 0 {
		e.Digits = 6
	}
	if e.Period == 0 {
		e.Period = 30
	}
	if strings.TrimSpace(e.Title) == "" {
		e.Title = e.Account
	}
	return s.v.SaveOTP(e)
}

// Delete removes an OTP entry.
func (s *OTPService) Delete(id int64) error {
	return s.v.DeleteOTP(id)
}

// ImportQRImage reads an image file (PNG/JPEG/GIF, e.g. a Google
// Authenticator export screenshot), decodes the QR code inside and stores
// what it contains. Both single otpauth:// URIs and batch
// otpauth-migration:// exports are supported; the latter returns the number
// of imported entries.
func (s *OTPService) ImportQRImage(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, errText("read image: " + err.Error())
	}
	text, err := otp.ParseQRImage(data)
	if err != nil {
		return 0, errText("识别二维码失败: " + err.Error())
	}
	return s.importDecoded(text)
}

// ImportURI parses an otpauth:// or otpauth-migration:// URI and stores it.
// Migration URIs import every entry in the batch; the returned count is the
// number of entries created.
func (s *OTPService) ImportURI(raw string) (int64, error) {
	return s.importDecoded(raw)
}

// ExportMigrationURIs packs all (or the selected) entries into
// otpauth-migration:// URIs, batching at most 10 entries per URI so each QR
// stays scannable. The result is a list of {uri, count} batches in list
// order.
func (s *OTPService) ExportMigrationURIs(ids []int64) ([]MigrationExportBatch, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	entries, err := s.exportEntries(ids)
	if err != nil {
		return nil, err
	}
	batches := (len(entries) + 9) / 10
	out := make([]MigrationExportBatch, 0, batches)
	for i := 0; i < len(entries); i += 10 {
		chunk := entries[i : i+min(10, len(entries)-i)]
		uri, err := otp.BuildMigrationURI(chunk, len(out), batches)
		if err != nil {
			return nil, errText("生成迁移数据失败: " + err.Error())
		}
		out = append(out, MigrationExportBatch{URI: uri, Count: len(chunk), Index: len(out) + 1, Total: batches})
	}
	return out, nil
}

// ExportEntryURI renders one entry as a standard otpauth:// URI.
func (s *OTPService) ExportEntryURI(id int64) (string, error) {
	if s.v.Locked() {
		return "", vault.ErrLocked
	}
	e, err := s.v.GetOTP(id)
	if err != nil {
		return "", err
	}
	uri, err := otp.BuildEntryURI(e)
	if err != nil {
		return "", errText("生成 URI 失败: " + err.Error())
	}
	return uri, nil
}

// ExportQRImage renders a migration URI as a PNG QR code and returns the
// bytes; the frontend saves it via ExportService or shows it inline.
func (s *OTPService) ExportQRImage(uri string) ([]byte, error) {
	png, err := otp.BuildMigrationQR(uri)
	if err != nil {
		return nil, errText("生成二维码失败: " + err.Error())
	}
	return png, nil
}

// ExportPlaintext renders the selected entries (nil = all) as a human-readable
// recovery sheet: title, account, otpauth:// URI and the Base32 secret. This
// is a plaintext export meant for offline safekeeping (e.g. printing), so the
// caller must treat the output as sensitive.
func (s *OTPService) ExportPlaintext(ids []int64) (string, error) {
	if s.v.Locked() {
		return "", vault.ErrLocked
	}
	entries, err := s.exportEntries(ids)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# WorkTools OTP 恢复码\n")
	b.WriteString("# 生成时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	b.WriteString("# ⚠️ 本文件包含明文密钥，仅用于离线备份/打印，请注意妥善保管！\n")
	b.WriteString("# 账户格式: 标题 | 账号 | 类型 | Base32 密钥 | otpauth URI\n")
	b.WriteString("# ============================================================\n\n")
	for _, e := range entries {
		uri, err := otp.BuildEntryURI(e)
		if err != nil {
			uri = "(无法生成 URI: " + err.Error() + ")"
		}
		account := e.Account
		if account == "" {
			account = "-"
		}
		typ := "TOTP"
		if strings.EqualFold(e.Type, "hotp") {
			typ = "HOTP"
		}
		title := e.Title
		if title == "" {
			title = "(未命名)"
		}
		b.WriteString("标题: " + title + "\n")
		if e.Issuer != "" {
			b.WriteString("发行方: " + e.Issuer + "\n")
		}
		b.WriteString("账号: " + account + "\n")
		b.WriteString("类型: " + typ + "\n")
		b.WriteString("密钥: " + e.Secret + "\n")
		b.WriteString("URI:  " + uri + "\n")
		b.WriteString("------------------------------------------------------------\n")
	}
	return b.String(), nil
}

// exportEntries resolves ids to full entries (with secrets); nil ids means
// all entries.
func (s *OTPService) exportEntries(ids []int64) ([]store.OTPEntry, error) {
	all, err := s.v.ListOTPs()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return all, nil
	}
	idset := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idset[id] = true
	}
	out := make([]store.OTPEntry, 0, len(ids))
	for _, e := range all {
		if idset[e.ID] {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, errText("没有可导出的条目")
	}
	return out, nil
}

// MigrationExportBatch is one otpauth-migration:// URI holding up to 10
// entries, plus batch position info for the UI.
type MigrationExportBatch struct {
	URI   string `json:"uri"`
	Count int    `json:"count"`
	Index int    `json:"index"` // 1-based
	Total int    `json:"total"`
}

// importDecoded stores entries from a decoded QR text or pasted URI.
func (s *OTPService) importDecoded(text string) (int64, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "otpauth-migration://") {
		res, err := otp.ParseMigrationURI(text)
		if err != nil {
			return 0, errText("解析迁移数据失败: " + err.Error())
		}
		var imported int64
		for _, e := range res.Entries {
			if _, err := s.Save(e); err != nil {
				return imported, errText("导入 " + e.Title + ": " + err.Error())
			}
			imported++
		}
		return imported, nil
	}
	e, err := otp.ParseURI(text)
	if err != nil {
		return 0, errText("二维码内容不是有效的 otpauth:// URI: " + err.Error())
	}
	return s.Save(e)
}

// Code returns the current code for the entry.
func (s *OTPService) Code(id int64) (otp.Code, error) {
	if s.v.Locked() {
		return otp.Code{}, vault.ErrLocked
	}
	e, err := s.v.GetOTP(id)
	if err != nil {
		return otp.Code{}, err
	}
	return otp.GenerateFor(e)
}

// NextCode advances the HOTP counter and returns the new code.
func (s *OTPService) NextCode(id int64) (otp.Code, error) {
	if s.v.Locked() {
		return otp.Code{}, vault.ErrLocked
	}
	if _, err := s.v.BumpOTPCounter(id); err != nil {
		return otp.Code{}, err
	}
	return s.Code(id)
}

// errText is a tiny helper to avoid importing errors in every file.
func errText(s string) error { return &textError{s} }

type textError struct{ s string }

func (e *textError) Error() string { return e.s }

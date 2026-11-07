package services

import (
	"context"
	"time"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/toolbox"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/vault"
)

// WorkbenchService powers the "daily use" features: cross-module quick search,
// password health audit, bulk URL health checks and external credential import.
type WorkbenchService struct {
	v *vault.Vault
}

// NewWorkbenchService wires the service to a vault.
func NewWorkbenchService(v *vault.Vault) *WorkbenchService {
	return &WorkbenchService{v: v}
}

// ---- 全局快速搜索 ----

// Search ranks every indexed item (URLs, secrets, OTP, notes, schedules and
// toolbox tools) against a free-text query.
func (s *WorkbenchService) Search(query string, limit int) ([]toolbox.SearchHit, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	docs, err := s.buildDocs()
	if err != nil {
		return nil, err
	}
	return toolbox.SearchDocs(query, docs, limit), nil
}

func (s *WorkbenchService) buildDocs() ([]toolbox.SearchDoc, error) {
	docs := make([]toolbox.SearchDoc, 0, 64)

	urls, err := s.v.ListURLs()
	if err != nil {
		return nil, err
	}
	for _, e := range urls {
		docs = append(docs, toolbox.SearchDoc{
			Kind: "url", ID: e.ID, Title: e.Title, Subtitle: e.URL,
			Body:  e.Env + " " + e.Tags + " " + e.Username + " " + e.Note,
			Route: "/urls",
		})
	}

	secrets, err := s.v.ListSecrets()
	if err != nil {
		return nil, err
	}
	for _, e := range secrets {
		sub := e.Host
		if e.Port > 0 {
			sub += ":" + itoaInt(e.Port)
		}
		docs = append(docs, toolbox.SearchDoc{
			Kind: "secret", ID: e.ID, Title: e.Title, Subtitle: sub,
			Body:  e.Category + " " + e.Username + " " + e.Note,
			Route: "/secrets",
		})
	}

	otps, err := s.v.ListOTPs()
	if err != nil {
		return nil, err
	}
	for _, e := range otps {
		docs = append(docs, toolbox.SearchDoc{
			Kind: "otp", ID: e.ID, Title: e.Title, Subtitle: e.Issuer + " " + e.Account,
			Route: "/otp",
		})
	}

	notes, err := s.v.ListNotes()
	if err != nil {
		return nil, err
	}
	for _, e := range notes {
		body := e.Content
		if len([]rune(body)) > 400 {
			body = string([]rune(body)[:400])
		}
		docs = append(docs, toolbox.SearchDoc{
			Kind: "note", ID: e.ID, Title: e.Title, Body: body, Route: "/notes",
		})
	}

	schedules, err := s.v.ListSchedules()
	if err != nil {
		return nil, err
	}
	for _, e := range schedules {
		docs = append(docs, toolbox.SearchDoc{
			Kind: "schedule", ID: e.ID, Title: e.Title, Subtitle: e.Detail, Route: "/schedule",
		})
	}

	docs = append(docs, toolboxTools()...)
	return docs, nil
}

// ---- 密码健康审计 ----

// PasswordAudit checks every stored credential for weak, reused or stale
// passwords.
func (s *WorkbenchService) PasswordAudit() (toolbox.PasswordAuditReport, error) {
	if s.v.Locked() {
		return toolbox.PasswordAuditReport{}, vault.ErrLocked
	}
	items := make([]toolbox.PasswordAuditItem, 0, 32)

	urls, err := s.v.ListURLs()
	if err != nil {
		return toolbox.PasswordAuditReport{}, err
	}
	for _, e := range urls {
		items = append(items, toolbox.PasswordAuditItem{
			ID: e.ID, Source: "url", Title: e.Title, Username: e.Username,
			Password: e.Password, UpdatedAt: e.UpdatedAt,
		})
	}

	secrets, err := s.v.ListSecrets()
	if err != nil {
		return toolbox.PasswordAuditReport{}, err
	}
	for _, e := range secrets {
		items = append(items, toolbox.PasswordAuditItem{
			ID: e.ID, Source: "secret", Title: e.Title, Username: e.Username,
			Password: e.Password, UpdatedAt: e.UpdatedAt,
		})
	}

	return toolbox.AuditPasswords(items, time.Now()), nil
}

// ---- URL 批量健康监测 ----

// URLHealthCheck probes every saved URL for reachability and certificate health.
func (s *WorkbenchService) URLHealthCheck(timeoutMS int) ([]toolbox.URLHealthResult, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	urls, err := s.v.ListURLs()
	if err != nil {
		return nil, err
	}
	items := make([]toolbox.URLHealthItem, 0, len(urls))
	for _, e := range urls {
		items = append(items, toolbox.URLHealthItem{ID: e.ID, URL: e.URL, Name: e.Title})
	}
	return toolbox.URLHealthCheck(context.Background(), items, timeoutMS), nil
}

// ---- 外部数据导入 ----

// ParseImport parses a KeePass / Chrome / 1Password CSV export without saving.
func (s *WorkbenchService) ParseImport(text, source string) (toolbox.ImportedCredentials, error) {
	return toolbox.ParseImportCSV(text, source)
}

// ImportCredentials writes parsed credentials into the vault. target is "urls"
// or "secrets". It returns the number of entries created.
func (s *WorkbenchService) ImportCredentials(items []toolbox.ImportedCredential, target string) (int, error) {
	if s.v.Locked() {
		return 0, vault.ErrLocked
	}
	n := 0
	for _, it := range items {
		if it.Title == "" && it.URL == "" && it.Password == "" {
			continue
		}
		switch target {
		case "secrets":
			_, err := s.v.SaveSecret(store.SecretEntry{
				Title: it.Title, Category: "imported", Username: it.Username,
				Password: it.Password, Note: it.Note,
			})
			if err != nil {
				return n, err
			}
		default:
			_, err := s.v.SaveURL(store.URLEntry{
				Title: it.Title, URL: it.URL, Username: it.Username,
				Password: it.Password, Note: it.Note,
			})
			if err != nil {
				return n, err
			}
		}
		n++
	}
	return n, nil
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// toolboxTools is the static catalogue of toolbox utilities for quick search.
func toolboxTools() []toolbox.SearchDoc {
	type entry struct{ name, route, keywords string }
	list := []entry{
		{"Base64 编解码", "/toolbox", "base64 encode decode 编码"},
		{"哈希计算", "/toolbox", "hash md5 sha1 sha256 摘要"},
		{"JSON 格式化", "/toolbox", "json format beautify 格式化"},
		{"URL 编解码", "/toolbox", "url encode decode 编码 转义"},
		{"JWT 解码", "/toolbox", "jwt token decode 令牌"},
		{"Cron 表达式", "/toolbox", "cron crontab 定时 表达式"},
		{"YAML 工具", "/toolbox", "yaml json format 校验"},
		{"时间差计算", "/toolbox", "time diff 时间戳 计算"},
		{"UUID / ULID 生成", "/toolbox", "uuid ulid 唯一 id 生成"},
		{"正则测试器", "/toolbox", "regex 正则 匹配"},
		{"文本 Diff", "/toolbox", "diff compare 文本 对比"},
		{"CSV 转 SQL", "/toolbox", "csv sql insert 转换"},
		{"二维码生成", "/toolbox", "qr 二维码 生成"},
		{"SSL 证书检查", "/toolbox", "ssl tls certificate 证书"},
		{"端口扫描", "/toolbox", "port scan 端口"},
		{"HTTP 探测", "/toolbox", "http probe 探测 状态"},
		{"TCPing", "/toolbox", "tcp ping 延迟"},
		{"WHOIS 查询", "/toolbox", "whois domain 域名"},
		{"自签证书生成", "/toolbox", "cert self-signed 自签 证书"},
		{"连接串解析", "/toolbox", "connstring dsn 连接串"},
		{"日志时间戳识别", "/toolbox", "log timestamp 日志 时间"},
		{"K8s 资源计算器", "/toolbox", "kubernetes k8s resource 资源 cpu memory"},
		{"DNS 查询", "/toolbox", "dns lookup 域名解析"},
		{"子网计算", "/toolbox", "subnet cidr 子网 掩码"},
		{"密码生成器", "/toolbox", "password generator 密码 生成"},
	}
	out := make([]toolbox.SearchDoc, 0, len(list))
	for i, e := range list {
		out = append(out, toolbox.SearchDoc{
			Kind: "tool", ID: int64(i + 1), Title: e.name, Body: e.keywords, Route: e.route,
		})
	}
	return out
}

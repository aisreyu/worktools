package toolbox

import (
	"context"
	"testing"
	"time"
)

func TestAuditPasswords(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	old := now.Add(-200 * 24 * time.Hour).Unix()
	fresh := now.Add(-2 * 24 * time.Hour).Unix()
	items := []PasswordAuditItem{
		{ID: 1, Source: "url", Title: "弱口令站点", Username: "u", Password: "123456", UpdatedAt: fresh},
		{ID: 2, Source: "url", Title: "复用站A", Username: "u", Password: "Str0ng!Passw0rd", UpdatedAt: fresh},
		{ID: 3, Source: "secret", Title: "复用站B", Username: "u", Password: "Str0ng!Passw0rd", UpdatedAt: old},
		{ID: 4, Source: "secret", Title: "空密码站", Username: "u", Password: "", UpdatedAt: fresh},
	}
	rep := AuditPasswords(items, now)
	if rep.Total != 4 {
		t.Fatalf("total = %d, want 4", rep.Total)
	}
	if rep.Weak < 2 {
		t.Errorf("weak = %d, want >= 2 (weak + empty)", rep.Weak)
	}
	if rep.Reused < 2 {
		t.Errorf("reused = %d, want >= 2", rep.Reused)
	}
	if rep.Old < 1 {
		t.Errorf("old = %d, want >= 1", rep.Old)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("expected findings")
	}
	// Findings are sorted high severity first.
	if rep.Findings[0].Severity != "high" {
		t.Errorf("first finding severity = %s, want high", rep.Findings[0].Severity)
	}
}

func TestSearchDocs(t *testing.T) {
	docs := []SearchDoc{
		{Kind: "url", ID: 1, Title: "Grafana 监控", Subtitle: "https://grafana.example.com", Route: "/urls"},
		{Kind: "secret", ID: 2, Title: "生产数据库", Body: "mysql grafana backup", Route: "/secrets"},
		{Kind: "note", ID: 3, Title: "部署笔记", Body: "grafana 端口 3000", Route: "/notes"},
		{Kind: "url", ID: 4, Title: "GitLab", Subtitle: "https://gitlab.example.com", Route: "/urls"},
	}
	hits := SearchDocs("grafana", docs, 10)
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want 3", len(hits))
	}
	// Title match should outrank body match.
	if hits[0].ID != 1 {
		t.Errorf("top hit = %d, want 1 (title match)", hits[0].ID)
	}
	if got := SearchDocs("gitlab", docs, 10); len(got) != 1 || got[0].ID != 4 {
		t.Errorf("gitlab search = %+v", got)
	}
	if got := SearchDocs("", docs, 2); len(got) != 2 {
		t.Errorf("empty query limit = %d, want 2", len(got))
	}
}

func TestParseImportCSV(t *testing.T) {
	csv := "name,url,username,password,notes\n" +
		"GitLab,https://gitlab.example.com,alice,s3cret,工作账号\n" +
		"Grafana,https://grafana.example.com,bob,p@ss,监控\n"
	got, err := ParseImportCSV(csv, "chrome")
	if err != nil {
		t.Fatalf("ParseImportCSV: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(got.Items))
	}
	if got.Items[0].Title != "GitLab" || got.Items[0].URL != "https://gitlab.example.com" ||
		got.Items[0].Username != "alice" || got.Items[0].Password != "s3cret" {
		t.Errorf("item0 = %+v", got.Items[0])
	}
	// 1Password style header with different aliases.
	csv2 := "Title,Username,Password,Login URI,Comments\nX,carol,pw,https://x.example.com,note\n"
	got2, err := ParseImportCSV(csv2, "1password")
	if err != nil {
		t.Fatalf("ParseImportCSV 1password: %v", err)
	}
	if len(got2.Items) != 1 || got2.Items[0].URL != "https://x.example.com" || got2.Items[0].Username != "carol" {
		t.Errorf("item = %+v", got2.Items)
	}
}

func TestURLHealthCheckUnreachable(t *testing.T) {
	items := []URLHealthItem{
		{ID: 1, URL: "http://127.0.0.1:1", Name: "closed"},
	}
	res := URLHealthCheck(context.Background(), items, 800)
	if len(res) != 1 {
		t.Fatalf("results = %d, want 1", len(res))
	}
	if res[0].OK && res[0].Error == "" {
		t.Errorf("expected failure for closed port, got %+v", res[0])
	}
}

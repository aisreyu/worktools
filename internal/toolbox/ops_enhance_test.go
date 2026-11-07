package toolbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExpandPortSpec(t *testing.T) {
	got, err := ExpandPortSpec("80,443,8000-8003,443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{80, 443, 8000, 8001, 8002, 8003}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if _, err := ExpandPortSpec("0"); err == nil {
		t.Fatal("expected error for port 0")
	}
	if _, err := ExpandPortSpec("1-100000"); err == nil {
		t.Fatal("expected error for out-of-range")
	}
	if _, err := ExpandPortSpec("1-99999"); err == nil {
		t.Fatal("expected error for oversized range")
	}
}

func TestPortPresetsNonEmpty(t *testing.T) {
	presets := PortPresets()
	if len(presets) == 0 {
		t.Fatal("expected presets")
	}
	for _, p := range presets {
		if p.Count == 0 {
			t.Fatalf("preset %s has no ports", p.Name)
		}
	}
}

func TestParseConnStringMySQL(t *testing.T) {
	c, err := ParseConnString("mysql://root:secret@10.0.0.5:3307/orders?charset=utf8mb4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Kind != "mysql" || c.Host != "10.0.0.5" || c.Port != 3307 || c.Database != "orders" {
		t.Fatalf("unexpected parse: %+v", c)
	}
	if c.Username != "root" || c.Password != "secret" {
		t.Fatalf("unexpected creds: %+v", c)
	}
	if c.Params["charset"] != "utf8mb4" {
		t.Fatalf("unexpected params: %+v", c.Params)
	}
}

func TestParseConnStringRedisDefaultPort(t *testing.T) {
	c, err := ParseConnString("redis://cache.internal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Port != 6379 {
		t.Fatalf("expected default redis port 6379, got %d", c.Port)
	}
	if c.Kind != "redis" {
		t.Fatalf("expected kind redis, got %s", c.Kind)
	}
}

func TestBuildConnStringRoundTrip(t *testing.T) {
	in := ConnString{Kind: "postgres", Host: "db.local", Port: 5432, Database: "app", Username: "u", Password: "p"}
	s, err := BuildConnString(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := ParseConnString(s)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if out.Host != "db.local" || out.Database != "app" || out.Password != "p" {
		t.Fatalf("round-trip mismatch: %+v (from %q)", out, s)
	}
}

func TestDetectLogTimeFormats(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"2023-04-05 12:30:45 INFO boot", "datetime"},
		{"2023-04-05T12:30:45Z INFO boot", "rfc3339"},
		{"10.0.0.1 - - [05/Apr/2023:12:30:45 +0800]", "apache"},
		{"1691234567890 hello", "epoch_ms"},
		{"1691234567 hello", "epoch_s"},
	}
	for _, c := range cases {
		if got := DetectLogTime(c.text).ID; got != c.want {
			t.Errorf("DetectLogTime(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestConvertLogTimeToEpoch(t *testing.T) {
	res, err := ConvertLogTime("ts=2023-04-05 12:30:45 done", "", "epoch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Count != 1 {
		t.Fatalf("expected 1 conversion, got %d", res.Count)
	}
	if strings.Contains(res.Converted, "2023-04-05") {
		t.Fatalf("timestamp not replaced: %q", res.Converted)
	}
	if res.Matches[0].Unix <= 0 {
		t.Fatalf("expected positive unix time, got %d", res.Matches[0].Unix)
	}
}

func TestParseQuantityCPU(t *testing.T) {
	q, err := ParseQuantity("500m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Kind != "cpu" || q.Value != 0.5 {
		t.Fatalf("unexpected quantity: %+v", q)
	}
	if FormatCPU(0.5) != "500m" {
		t.Fatalf("FormatCPU(0.5) = %q", FormatCPU(0.5))
	}
	if FormatCPU(2) != "2" {
		t.Fatalf("FormatCPU(2) = %q", FormatCPU(2))
	}
}

func TestParseQuantityMemory(t *testing.T) {
	q, err := ParseQuantity("1.5Gi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Kind != "memory" || int64(q.Value) != int64(1536*(1<<20)) {
		t.Fatalf("unexpected quantity: %+v", q)
	}
	if FormatMemoryBytes(q.Value) != "1.500Gi" {
		t.Fatalf("FormatMemoryBytes = %q", FormatMemoryBytes(q.Value))
	}
	if _, err := ParseQuantity("abc"); err == nil {
		t.Fatal("expected error for bad quantity")
	}
}

func TestConvertK8sResource(t *testing.T) {
	got, err := ConvertK8sResource("1000m", "cpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.000" {
		t.Fatalf("got %q", got)
	}
	got, err = ConvertK8sResource("1Gi", "Mi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1024.000Mi" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateResourcesYAML(t *testing.T) {
	yaml := `
resources:
  requests:
    cpu: 250m
    memory: 512Mi
  limits:
    cpu: 500m
    memory: 1Gi
`
	res := ValidateResourcesYAML(yaml)
	if !res.OK {
		t.Fatalf("expected OK, issues: %v", res.Issues)
	}
	if len(res.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(res.Entries))
	}
	bad := `
  limits:
    cpu: 100m
  requests:
    cpu: 500m
`
	res2 := ValidateResourcesYAML(bad)
	if res2.OK {
		t.Fatal("expected limit<request to fail")
	}
}

func TestGenerateSelfSignedCert(t *testing.T) {
	res, err := GenerateSelfSignedCert(CertGenOptions{
		CommonName: "example.test",
		DNSNames:   []string{"example.test", "www.example.test"},
		IPs:        []string{"127.0.0.1"},
		Days:       30,
		KeyType:    "ecdsa256",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.CertPEM, "BEGIN CERTIFICATE") {
		t.Fatal("missing cert PEM")
	}
	if !strings.Contains(res.KeyPEM, "BEGIN PRIVATE KEY") {
		t.Fatal("missing key PEM")
	}
	info, err := CertPEMToInfo(res.CertPEM)
	if err != nil {
		t.Fatalf("failed to re-parse cert: %v", err)
	}
	if info.Subject == "" || !info.SelfSigned {
		t.Fatalf("unexpected cert info: %+v", info)
	}
}

func TestGenerateCSR(t *testing.T) {
	res, err := GenerateCSR(CertGenOptions{CommonName: "svc.local", KeyType: "rsa2048"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.CertPEM, "BEGIN CERTIFICATE REQUEST") {
		t.Fatal("missing CSR PEM")
	}
}

// The following hit real endpoints and are skipped when the network is down.

func TestHTTPRequestSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := HTTPRequest(ctx, HTTPRequestOptions{URL: "http://www.baidu.com", TimeoutMS: 8000})
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if res.Status == 0 || res.Latency == "" {
		t.Fatalf("unexpected empty result: %+v", res)
	}
}

func TestTCPingSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := TCPing(ctx, "www.baidu.com", 443, 2, 4000, 100)
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if res.Sent != 2 || len(res.Samples) != 2 {
		t.Fatalf("unexpected sample count: %+v", res)
	}
}

func TestWhoisSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	res, err := Whois(ctx, "example.com", 15000)
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if res.Raw == "" || res.Server == "" {
		t.Fatalf("unexpected empty whois result: %+v", res)
	}
}

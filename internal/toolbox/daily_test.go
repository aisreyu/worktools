package toolbox

import (
	"strings"
	"testing"
)

// ---- URL encode/decode ----

func TestURLEncodeDecode(t *testing.T) {
	in := "a b&c=d/e?f"
	enc := URLEncodeComponent(in, "query")
	if !strings.Contains(enc, "a+b") && !strings.Contains(enc, "a%20b") {
		t.Fatalf("query encoding unexpected: %q", enc)
	}
	got, err := URLDecodeComponent(enc, "query")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != in {
		t.Fatalf("roundtrip got %q want %q", got, in)
	}

	// path mode keeps "/" literal
	if p := URLEncodeComponent("a/b c", "path"); strings.Contains(p, "/") == false {
		t.Fatalf("path mode should keep slash: %q", p)
	}

	// decoding plain text is tolerated
	if d, err := URLDecodeComponent("plain", "query"); err != nil || d != "plain" {
		t.Fatalf("tolerant decode got %q err %v", d, err)
	}
}

// ---- JWT ----

func TestDecodeJWT(t *testing.T) {
	// header {"alg":"HS256","typ":"JWT"}, payload {"sub":"1","exp":9999999999}
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxIiwiZXhwIjo5OTk5OTk5OTk5fQ." +
		"c2lnbmF0dXJl"
	r, err := DecodeJWT("Bearer " + token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !r.Valid {
		t.Fatal("expected valid")
	}
	if r.Alg != "HS256" {
		t.Fatalf("alg got %q", r.Alg)
	}
	if r.Payload["sub"] != "1" {
		t.Fatalf("sub got %v", r.Payload["sub"])
	}
	if r.Expired {
		t.Fatal("token should not be expired")
	}
	if _, err := DecodeJWT("not-a-token"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}

// ---- Cron ----

func TestParseCron(t *testing.T) {
	s, err := ParseCron("*/15 9-17 * * mon-fri", 5)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.NextRuns) != 5 {
		t.Fatalf("expected 5 preview runs, got %d", len(s.NextRuns))
	}
	macro, err := ParseCron("@daily", 3)
	if err != nil {
		t.Fatalf("macro parse: %v", err)
	}
	if len(macro.NextRuns) != 3 {
		t.Fatalf("expected 3 macro runs, got %d", len(macro.NextRuns))
	}
	if _, err := ParseCron("bad expr", 1); err == nil {
		t.Fatal("expected error for bad expr")
	}
}

// ---- YAML ----

func TestYAMLJSONRoundtrip(t *testing.T) {
	y := "a: 1\nb:\n  - x\n  - y\n"
	j, err := YAMLToJSON(y)
	if err != nil {
		t.Fatalf("yaml->json: %v", err)
	}
	if !strings.Contains(j, "\"a\": 1") {
		t.Fatalf("json = %s", j)
	}
	back, err := JSONToYAML(j)
	if err != nil {
		t.Fatalf("json->yaml: %v", err)
	}
	if !strings.Contains(back, "a: 1") {
		t.Fatalf("yaml = %s", back)
	}
	if err := ValidateYAML("a: [1,2"); err == nil {
		t.Fatal("expected error for invalid yaml")
	}
	if f, err := FormatYAML("a:   1"); err != nil || !strings.Contains(f, "a: 1") {
		t.Fatalf("format got %q err %v", f, err)
	}
}

// ---- time diff ----

func TestTimeDiff(t *testing.T) {
	r, err := TimeDiff("2024-01-01 00:00:00", "2024-01-02 03:04:05")
	if err != nil {
		t.Fatalf("timediff: %v", err)
	}
	if r.Seconds != 97445 {
		t.Fatalf("seconds got %d", r.Seconds)
	}
	if !strings.Contains(r.Humanized, "1d") || !strings.Contains(r.Humanized, "3h") {
		t.Fatalf("humanized = %q", r.Humanized)
	}
	neg, err := TimeDiff("2024-01-02", "2024-01-01")
	if err != nil {
		t.Fatalf("timediff neg: %v", err)
	}
	if neg.Seconds >= 0 {
		t.Fatalf("expected negative, got %d", neg.Seconds)
	}
}

// ---- identifiers ----

func TestUUIDAndULID(t *testing.T) {
	us, err := UUIDs(3)
	if err != nil {
		t.Fatalf("uuids: %v", err)
	}
	if len(us) != 3 || len(us[0]) != 36 {
		t.Fatalf("uuids = %v", us)
	}
	ul, err := ULID()
	if err != nil {
		t.Fatalf("ulid: %v", err)
	}
	if len(ul) != 26 || !ValidateULID(ul) {
		t.Fatalf("ulid invalid: %q", ul)
	}
	if ValidateULID("!!!!") {
		t.Fatal("expected invalid ulid")
	}
}

// ---- regex ----

func TestRegexTest(t *testing.T) {
	r := RegexTest(`(\d+)-(\d+)`, "a 12-34 b 5-6", false)
	if r.Error != "" {
		t.Fatalf("unexpected error: %s", r.Error)
	}
	if r.Count != 2 {
		t.Fatalf("count = %d", r.Count)
	}
	if r.Matches[0].Groups[1] != "12" {
		t.Fatalf("group1 = %q", r.Matches[0].Groups[1])
	}
	named := RegexTest(`(?P<year>\d{4})`, "in 2024", false)
	if named.Matches[0].Named["year"] != "2024" {
		t.Fatalf("named = %v", named.Matches[0].Named)
	}
	bad := RegexTest("(", "x", false)
	if bad.Error == "" {
		t.Fatal("expected regex error")
	}
	out, err := RegexReplace(`\d+`, "a1b2", "#", false)
	if err != nil || out != "a#b#" {
		t.Fatalf("replace got %q err %v", out, err)
	}
}

// ---- text diff ----

func TestTextDiff(t *testing.T) {
	r := TextDiff("a\nb\nc", "a\nx\nc")
	if r.Added != 1 || r.Deleted != 1 {
		t.Fatalf("added=%d deleted=%d", r.Added, r.Deleted)
	}
	if r.Same != 2 {
		t.Fatalf("same=%d", r.Same)
	}
	u := UnifiedDiff("a\nb", "a\nc")
	if !strings.Contains(u, "- b") || !strings.Contains(u, "+ c") {
		t.Fatalf("unified = %q", u)
	}
}

// ---- csv to sql ----

func TestCSVToSQL(t *testing.T) {
	csv := "id,name,active\n1,alice,true\n2,bob's,false"
	out, err := CSVToSQL(csv, "users", "postgres", true)
	if err != nil {
		t.Fatalf("csv->sql: %v", err)
	}
	if !strings.Contains(out, `CREATE TABLE "users"`) {
		t.Fatalf("missing create: %s", out)
	}
	if !strings.Contains(out, `INSERT INTO "users" ("id", "name", "active") VALUES (1, 'alice', 'true')`) {
		t.Fatalf("missing insert: %s", out)
	}
	if !strings.Contains(out, "'bob''s'") {
		t.Fatalf("quote escaping failed: %s", out)
	}
	mys, err := CSVToSQL(csv, "users", "mysql", false)
	if err != nil || !strings.Contains(mys, "`users`") {
		t.Fatalf("mysql dialect: %v %s", err, mys)
	}
	if _, err := CSVToSQL(csv, "", "postgres", false); err == nil {
		t.Fatal("expected error for empty table")
	}
}

// ---- qr ----

func TestQRGenerate(t *testing.T) {
	b64, err := QRGenerate("https://example.com", "M", 4)
	if err != nil {
		t.Fatalf("qr: %v", err)
	}
	if len(b64) < 100 {
		t.Fatalf("qr too small: %d bytes", len(b64))
	}
	if _, err := QRGenerate("", "M", 4); err == nil {
		t.Fatal("expected error for empty text")
	}
}

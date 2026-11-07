package otp

import (
	"encoding/base32"
	"net/url"
	"strings"
	"testing"

	"github.com/aisreyu/worktools/internal/store"
)

func TestBuildMigrationURIRoundTrip(t *testing.T) {
	entries := []store.OTPEntry{
		{Title: "GitHub", Issuer: "GitHub", Account: "alice@example.com", Secret: "JBSWY3DPEHPK3PXP", Algorithm: "SHA1", Digits: 6, Period: 30, Type: "totp"},
		{Title: "Server", Issuer: "Ops", Account: "root@srv", Secret: "KRSXG5CTMVRXEZLU", Algorithm: "SHA256", Digits: 8, Period: 30, Type: "hotp", Counter: 5},
	}
	uri, err := BuildMigrationURI(entries, 0, 1)
	if err != nil {
		t.Fatalf("BuildMigrationURI: %v", err)
	}
	if !strings.HasPrefix(uri, "otpauth-migration://offline?data=") {
		t.Fatalf("unexpected prefix: %s", uri)
	}
	res, err := ParseMigrationURI(uri)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(res.Entries))
	}
	got := res.Entries[0]
	if got.Issuer != "GitHub" || got.Account != "alice@example.com" {
		t.Errorf("entry0 issuer/account = %q/%q", got.Issuer, got.Account)
	}
	if got.Secret != "JBSWY3DPEHPK3PXP" || got.Algorithm != "SHA1" || got.Digits != 6 || got.Type != "totp" {
		t.Errorf("entry0 mismatch: %+v", got)
	}
	h := res.Entries[1]
	if h.Algorithm != "SHA256" || h.Digits != 8 || h.Type != "hotp" || h.Counter != 5 {
		t.Errorf("entry1 mismatch: %+v", h)
	}
	if res.Version != 1 || res.BatchSize != 1 || res.BatchIndex != 0 {
		t.Errorf("batch meta mismatch: %+v", res)
	}
}

func TestBuildMigrationURILimits(t *testing.T) {
	if _, err := BuildMigrationURI(nil, 0, 1); err == nil {
		t.Fatal("empty entries should error")
	}
	many := make([]store.OTPEntry, 11)
	for i := range many {
		many[i] = store.OTPEntry{Title: "x", Secret: "JBSWY3DPEHPK3PXP"}
	}
	if _, err := BuildMigrationURI(many, 0, 1); err == nil {
		t.Fatal("11 entries should error")
	}
}

func TestBuildMigrationQR(t *testing.T) {
	uri, err := BuildMigrationURI([]store.OTPEntry{
		{Title: "T", Issuer: "T", Account: "a", Secret: "JBSWY3DPEHPK3PXP"},
	}, 0, 1)
	if err != nil {
		t.Fatalf("BuildMigrationURI: %v", err)
	}
	png, err := BuildMigrationQR(uri)
	if err != nil {
		t.Fatalf("BuildMigrationQR: %v", err)
	}
	// The QR must decode back to the same URI through the existing decoder.
	text, err := ParseQRImage(png)
	if err != nil {
		t.Fatalf("ParseQRImage of generated QR: %v", err)
	}
	if text != uri {
		t.Errorf("decoded text != original uri:\n%q\n%q", text, uri)
	}
}

func TestBuildEntryURI(t *testing.T) {
	e := store.OTPEntry{Title: "GitHub", Issuer: "GitHub", Account: "alice", Secret: "JBSWY3DPEHPK3PXP", Algorithm: "SHA1", Digits: 6, Period: 30, Type: "totp"}
	uri, err := BuildEntryURI(e)
	if err != nil {
		t.Fatalf("BuildEntryURI: %v", err)
	}
	back, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("ParseURI round-trip: %v", err)
	}
	if back.Issuer != "GitHub" || back.Account != "alice" || back.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

func TestPaddedSecretRoundTrip(t *testing.T) {
	// Secrets imported from migration QR codes are stored with standard
	// base32 padding (e.g. 16-byte keys → 26 chars + "===="). Export must
	// accept them, not fail with "illegal base32 data at input byte 26".
	raw := []byte("0123456789abcdef")
	padded := base32.StdEncoding.EncodeToString(raw) // 26 chars + "===="
	if !strings.HasSuffix(padded, "=") {
		t.Fatalf("expected padded encoding, got %q", padded)
	}
	entries := []store.OTPEntry{
		{Title: "padded", Issuer: "X", Account: "a", Secret: padded, Algorithm: "SHA1", Digits: 6, Period: 30, Type: "totp"},
	}
	uri, err := BuildMigrationURI(entries, 0, 1)
	if err != nil {
		t.Fatalf("BuildMigrationURI with padded secret: %v", err)
	}
	res, err := ParseMigrationURI(uri)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(res.Entries))
	}
	if res.Entries[0].Secret != padded {
		t.Errorf("secret mismatch: %q != %q", res.Entries[0].Secret, padded)
	}

	// Single-entry URI export must also normalise padding away.
	e := entries[0]
	single, err := BuildEntryURI(e)
	if err != nil {
		t.Fatalf("BuildEntryURI with padded secret: %v", err)
	}
	if u, err := url.Parse(single); err != nil {
		t.Fatalf("parse single uri: %v", err)
	} else if strings.Contains(u.Query().Get("secret"), "=") {
		t.Errorf("secret in uri should not contain padding: %s", single)
	}
	back, err := ParseURI(single)
	if err != nil {
		t.Fatalf("ParseURI of normalised uri: %v", err)
	}
	if back.Secret != strings.TrimRight(padded, "=") {
		t.Errorf("round-trip secret mismatch: %q", back.Secret)
	}
}

package otp

import (
	"encoding/base32"
	"encoding/base64"
	"testing"
)

// Minimal protobuf encoder helpers for building test payloads.

func putVarintField(b []byte, field int, v uint64) []byte {
	b = append(b, byte(field<<3|0))
	return putUvarint(b, v)
}

func putBytesField(b []byte, field int, v []byte) []byte {
	b = append(b, byte(field<<3|2))
	b = putUvarint(b, uint64(len(v)))
	return append(b, v...)
}

func putUvarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func TestParseMigrationURI(t *testing.T) {
	secret1 := []byte("JBSWY3DP") // raw bytes, base32-encoded by parser
	secret2 := []byte("KRSXG5CT")

	// OtpParameters{secret=1, name=2, issuer=3, algorithm=4, digits=5, type=6, counter=7}
	p1 := putBytesField(nil, 1, secret1)
	p1 = putBytesField(p1, 2, []byte("GitHub:alice@example.com"))
	p1 = putBytesField(p1, 3, []byte("GitHub"))
	p1 = putVarintField(p1, 4, 1) // SHA1
	p1 = putVarintField(p1, 5, 1) // 6 digits
	p1 = putVarintField(p1, 6, 0) // TOTP

	p2 := putBytesField(nil, 1, secret2)
	p2 = putBytesField(p2, 2, []byte("AWS"))
	p2 = putVarintField(p2, 4, 2) // SHA256
	p2 = putVarintField(p2, 5, 2) // 8 digits
	p2 = putVarintField(p2, 6, 1) // HOTP
	p2 = putVarintField(p2, 7, 5) // counter

	payload := putBytesField(nil, 1, p1)
	payload = putBytesField(payload, 1, p2)
	payload = putVarintField(payload, 2, 1) // version
	payload = putVarintField(payload, 3, 2) // batch_size
	payload = putVarintField(payload, 4, 0) // batch_index
	payload = putVarintField(payload, 5, 42)

	data := base64.RawURLEncoding.EncodeToString(payload)
	uri := "otpauth-migration://offline?data=" + data

	res, err := ParseMigrationURI(uri)
	if err != nil {
		t.Fatalf("ParseMigrationURI: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(res.Entries))
	}
	if res.BatchSize != 2 || res.BatchID != 42 {
		t.Fatalf("batch meta mismatch: %+v", res)
	}

	e1 := res.Entries[0]
	if e1.Issuer != "GitHub" || e1.Account != "alice@example.com" {
		t.Errorf("entry1 issuer/account = %q/%q", e1.Issuer, e1.Account)
	}
	if e1.Secret != base32.StdEncoding.EncodeToString(secret1) {
		t.Errorf("entry1 secret = %q", e1.Secret)
	}
	if e1.Type != "totp" || e1.Algorithm != "SHA1" || e1.Digits != 6 {
		t.Errorf("entry1 params = %+v", e1)
	}

	e2 := res.Entries[1]
	// No issuer field and no colon in name: account keeps the full name,
	// issuer stays empty.
	if e2.Issuer != "" || e2.Account != "AWS" {
		t.Errorf("entry2 issuer/account = %q/%q", e2.Issuer, e2.Account)
	}
	if e2.Type != "hotp" || e2.Algorithm != "SHA256" || e2.Digits != 8 || e2.Counter != 5 {
		t.Errorf("entry2 params = %+v", e2)
	}
}

func TestParseMigrationURIIssuerFromName(t *testing.T) {
	// No issuer field; name is "Issuer:Account" — parser should split it.
	p := putBytesField(nil, 1, []byte("JBSWY3DP"))
	p = putBytesField(p, 2, []byte("GitLab:bob"))
	payload := putBytesField(nil, 1, p)
	uri := "otpauth-migration://offline?data=" + base64.URLEncoding.EncodeToString(payload)

	res, err := ParseMigrationURI(uri)
	if err != nil {
		t.Fatalf("ParseMigrationURI: %v", err)
	}
	e := res.Entries[0]
	if e.Issuer != "GitLab" || e.Account != "bob" {
		t.Errorf("issuer/account = %q/%q", e.Issuer, e.Account)
	}
}

func TestParseMigrationURIRejectsBadInput(t *testing.T) {
	cases := []string{
		"otpauth://totp/Example:alice?secret=JBSW",
		"otpauth-migration://offline",
		"otpauth-migration://offline?data=!!!not-base64!!!",
	}
	for _, c := range cases {
		if _, err := ParseMigrationURI(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseMigrationURIMultiBatch(t *testing.T) {
	// Simulate the second QR of a 3-batch export.
	p := putBytesField(nil, 1, []byte("JBSWY3DP"))
	p = putBytesField(p, 2, []byte("svc"))
	payload := putBytesField(nil, 1, p)
	payload = putVarintField(payload, 3, 3) // batch_size
	payload = putVarintField(payload, 4, 1) // batch_index
	uri := "otpauth-migration://offline?data=" + base64.RawURLEncoding.EncodeToString(payload)

	res, err := ParseMigrationURI(uri)
	if err != nil {
		t.Fatalf("ParseMigrationURI: %v", err)
	}
	if res.BatchIndex != 1 || res.BatchSize != 3 {
		t.Fatalf("batch = %+v", res)
	}
	if want := base32.StdEncoding.EncodeToString([]byte("JBSWY3DP")); res.Entries[0].Secret != want {
		t.Errorf("secret = %q, want %q", res.Entries[0].Secret, want)
	}
}

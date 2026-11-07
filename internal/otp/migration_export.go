package otp

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image/png"
	"net/url"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"

	"github.com/aisreyu/worktools/internal/store"
)

// BuildMigrationURI encodes entries into an otpauth-migration:// URI, the
// format Google Authenticator produces on "Export accounts" and most other
// authenticator apps (Aegis, Authy, Microsoft Authenticator, FreeOTP, ...)
// can import by scanning. Entries are chunked into batches of up to 10 to
// keep each QR code scannable.
func BuildMigrationURI(entries []store.OTPEntry, batchIndex, batchSize int) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("no entries to export")
	}
	if len(entries) > 10 {
		return "", fmt.Errorf("a migration batch holds at most 10 entries")
	}
	var buf bytes.Buffer
	for _, e := range entries {
		p, err := encodeOtpParameters(e)
		if err != nil {
			return "", err
		}
		appendBytesField(&buf, 1, p)
	}
	appendVarintField(&buf, 2, 1) // version
	if batchSize > 0 {
		appendVarintField(&buf, 3, uint64(batchSize))
		appendVarintField(&buf, 4, uint64(batchIndex))
		appendVarintField(&buf, 5, uint64(migrationBatchID))
	}
	data := base64.RawURLEncoding.EncodeToString(buf.Bytes())
	q := url.Values{}
	q.Set("data", data)
	return "otpauth-migration://offline?" + q.Encode(), nil
}

// migrationBatchID identifies one export session; fixed is fine for a
// desktop tool where the user exports once.
const migrationBatchID = 1

// decodeSecretBase32 accepts both padded and unpadded base32 secrets —
// entries imported from migration QR codes are stored padded while manual
// entries are typically unpadded.
func decodeSecretBase32(s string) ([]byte, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if s == "" {
		return nil, fmt.Errorf("empty secret")
	}
	if b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s); err == nil {
		return b, nil
	}
	// Strip padding and retry so padded secrets ("...==") also decode.
	s = strings.TrimRight(s, "=")
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

// encodeOtpParameters serialises one entry as a protobuf OtpParameters msg.
func encodeOtpParameters(e store.OTPEntry) ([]byte, error) {
	secret, err := decodeSecretBase32(e.Secret)
	if err != nil {
		return nil, fmt.Errorf("entry %q: secret is not valid base32: %w", e.Title, err)
	}
	var b bytes.Buffer
	appendBytesField(&b, 1, secret)

	name := e.Account
	if e.Issuer != "" {
		name = e.Issuer + ":" + e.Account
	}
	if name == "" {
		name = e.Title
	}
	appendBytesField(&b, 2, []byte(name))
	if e.Issuer != "" {
		appendBytesField(&b, 3, []byte(e.Issuer))
	}

	switch strings.ToUpper(e.Algorithm) {
	case "SHA256":
		appendVarintField(&b, 4, 2)
	case "SHA512":
		appendVarintField(&b, 4, 3)
	default: // SHA1
		appendVarintField(&b, 4, 1)
	}
	if e.Digits == 8 {
		appendVarintField(&b, 5, 2)
	} else {
		appendVarintField(&b, 5, 1)
	}
	if strings.EqualFold(e.Type, "hotp") {
		appendVarintField(&b, 6, 1)
		appendVarintField(&b, 7, uint64(e.Counter))
	} else {
		appendVarintField(&b, 6, 0)
	}
	return b.Bytes(), nil
}

func appendVarintField(buf *bytes.Buffer, field int, v uint64) {
	writeTag(buf, field, 0)
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], v)
	buf.Write(tmp[:n])
}

func appendBytesField(buf *bytes.Buffer, field int, v []byte) {
	writeTag(buf, field, 2)
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64(len(v)))
	buf.Write(tmp[:n])
	buf.Write(v)
}

func writeTag(buf *bytes.Buffer, field, wireType int) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64(field)<<3|uint64(wireType))
	buf.Write(tmp[:n])
}

// BuildMigrationQR renders a migration URI as a PNG QR code image.
func BuildMigrationQR(uri string) ([]byte, error) {
	code, err := qr.Encode(uri, qr.M, qr.Unicode)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}
	// Scale up: scanners struggle with 1px-per-module codes on screen.
	w := code.Bounds().Dx()
	scaled, err := barcode.Scale(code, w*8, w*8)
	if err != nil {
		return nil, fmt.Errorf("scale qr: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, fmt.Errorf("render qr png: %w", err)
	}
	return buf.Bytes(), nil
}

// BuildEntryURI renders a single entry as a standard otpauth:// URI that any
// authenticator app can scan.
func BuildEntryURI(e store.OTPEntry) (string, error) {
	secret, err := decodeSecretBase32(e.Secret)
	if err != nil {
		return "", fmt.Errorf("entry %q: secret is not valid base32: %w", e.Title, err)
	}
	secretB32 := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	label := e.Account
	if label == "" {
		label = e.Title
	}
	if e.Issuer != "" {
		label = e.Issuer + ":" + label
	}
	typ := "totp"
	if strings.EqualFold(e.Type, "hotp") {
		typ = "hotp"
	}
	q := url.Values{}
	q.Set("secret", secretB32)
	if e.Issuer != "" {
		q.Set("issuer", e.Issuer)
	}
	algo := strings.ToUpper(strings.TrimSpace(e.Algorithm))
	if algo == "" {
		algo = "SHA1"
	}
	q.Set("algorithm", algo)
	if e.Digits == 8 {
		q.Set("digits", "8")
	}
	if typ == "totp" && e.Period > 0 && e.Period != 30 {
		q.Set("period", fmt.Sprintf("%d", e.Period))
	}
	if typ == "hotp" {
		q.Set("counter", fmt.Sprintf("%d", e.Counter))
	}
	return (&url.URL{
		Scheme:   "otpauth",
		Host:     typ,
		Path:     "/" + label,
		RawQuery: q.Encode(),
	}).String(), nil
}

package otp

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// makeQR renders text into a QR PNG, mimicking a Google Authenticator export.
func makeQR(t *testing.T, text string) []byte {
	t.Helper()
	w := qrcode.NewQRCodeWriter()
	bmp, err := w.EncodeWithoutHint(text, gozxing.BarcodeFormat_QR_CODE, 200, 200)
	if err != nil {
		t.Fatalf("encode qr: %v", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, bmp); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func TestParseQRImage(t *testing.T) {
	uri := "otpauth://totp/GitHub:alice@example.com?secret=JBSWY3DPEHPK3PXP&issuer=GitHub&algorithm=SHA1&digits=6&period=30"
	data := makeQR(t, uri)

	got, err := ParseQRImage(data)
	if err != nil {
		t.Fatalf("ParseQRImage: %v", err)
	}
	if got != uri {
		t.Fatalf("decoded text mismatch:\n got: %s\nwant: %s", got, uri)
	}

	e, err := ParseURI(got)
	if err != nil {
		t.Fatalf("ParseURI on decoded text: %v", err)
	}
	if e.Secret != "JBSWY3DPEHPK3PXP" || e.Issuer != "GitHub" || e.Account != "alice@example.com" {
		t.Fatalf("entry mismatch: %+v", e)
	}
}

func TestParseQRImageInvalid(t *testing.T) {
	if _, err := ParseQRImage([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image bytes")
	}
	// A QR containing something other than an otpauth URI decodes fine but
	// must fail URI parsing downstream.
	data := makeQR(t, "https://example.com/hello")
	if _, err := ParseQRImage(data); err != nil {
		t.Fatalf("decode should succeed: %v", err)
	}
}

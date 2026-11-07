package toolbox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

// QRGenerate renders text as a PNG QR code and returns it base64-encoded
// (standard, no data-URI prefix). ecLevel is one of "L","M","Q","H"; scale is
// the pixel multiplier per module (defaults to 8).
func QRGenerate(text, ecLevel string, scale int) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	if scale <= 0 {
		scale = 8
	}
	level := qr.M
	switch ecLevel {
	case "L", "l":
		level = qr.L
	case "Q", "q":
		level = qr.Q
	case "H", "h":
		level = qr.H
	}
	code, err := qr.Encode(text, level, qr.Unicode)
	if err != nil {
		return "", fmt.Errorf("encode qr: %w", err)
	}
	w := code.Bounds().Dx()
	scaled, err := barcode.Scale(code, w*scale, w*scale)
	if err != nil {
		return "", fmt.Errorf("scale qr: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("render qr png: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

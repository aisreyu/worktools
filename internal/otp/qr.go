package otp

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/disintegration/imaging"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// ParseQRImage decodes a QR code from raw image bytes (PNG/JPEG/GIF) and
// returns the embedded text. Google Authenticator exports otpauth:// URIs as
// QR images, so the result usually feeds straight into ParseURI.
//
// Small or oddly-scaled screenshots often fail to decode, so the image is
// retried at several scales before giving up.
func ParseQRImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	text, err := decodeQR(img)
	if err == nil {
		return text, nil
	}

	// Retry with scaled variants: screenshots of QR codes are frequently
	// too small or have anti-aliasing artefacts that confuse the detector.
	for _, scale := range []float64{2, 0.5, 3, 1.5} {
		w := int(float64(img.Bounds().Dx()) * scale)
		h := int(float64(img.Bounds().Dy()) * scale)
		if w < 20 || h < 20 || w > 4000 || h > 4000 {
			continue
		}
		scaled := imaging.Resize(img, w, h, imaging.Lanczos)
		if t, e := decodeQR(scaled); e == nil {
			return t, nil
		}
	}

	// Last resort: grayscale + contrast boost helps with photos.
	gray := imaging.Grayscale(img)
	enhanced := imaging.AdjustFunc(gray, func(c color.NRGBA) color.NRGBA {
		if c.R < 80 {
			return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		}
		if c.R > 160 {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		return c
	})
	return decodeQR(enhanced)
}

// decodeQR runs the gozxing QR detector over an image.
func decodeQR(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	})
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

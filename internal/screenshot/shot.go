package screenshot

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
)

// Shot is a captured image plus its metadata.
type Shot struct {
	Data   []byte  `json:"-"` // raw PNG bytes
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Scale  float64 `json:"scale"`
}

// capture grabs the rectangle r (DIP) and returns a decoded PNG. scale is the
// display scale factor used by the pixel-based backends.
func capture(r Rect, scale float64) (Shot, error) {
	data, err := captureRectRaw(r, scale)
	if err != nil {
		return Shot{}, err
	}
	w, h := pngSize(data)
	if w == 0 || h == 0 {
		// Fall back to a full decode when the header parse fails.
		img, _, decErr := image.Decode(bytes.NewReader(data))
		if decErr != nil {
			return Shot{}, fmt.Errorf("decode capture: %w", decErr)
		}
		b := img.Bounds()
		w, h = b.Dx(), b.Dy()
	}
	return Shot{Data: data, Width: w, Height: h, Scale: scale}, nil
}

// captureB64 grabs a rect and returns the PNG as base64.
func captureB64(r Rect, scale float64) (Shot, string, error) {
	shot, err := capture(r, scale)
	if err != nil {
		return Shot{}, "", err
	}
	return shot, base64.StdEncoding.EncodeToString(shot.Data), nil
}

// pngSize reads the pixel dimensions from a PNG stream header. It returns
// (0, 0) when the payload is not a decodable PNG.
func pngSize(png []byte) (int, int) {
	if len(png) < 24 || !bytes.Equal(png[:8], pngSignature) || string(png[12:16]) != "IHDR" {
		return 0, 0
	}
	w := int(binary.BigEndian.Uint32(png[16:20]))
	h := int(binary.BigEndian.Uint32(png[20:24]))
	return w, h
}

// pngSignature is the 8-byte PNG file header.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// encodePNG encodes an image to PNG bytes.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// base64PNG encodes raw PNG bytes as standard base64.
func base64PNG(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// CropPNG crops the PNG in data to the rectangle (x, y, w, h) in image pixels
// and returns the cropped PNG. Coordinates are clamped to the image bounds.
func CropPNG(data []byte, x, y, w, h int) ([]byte, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}
	b := img.Bounds()
	// Clamp to the image rectangle.
	x0 := clamp(x, 0, b.Dx())
	y0 := clamp(y, 0, b.Dy())
	x1 := clamp(x+w, 0, b.Dx())
	y1 := clamp(y+h, 0, b.Dy())
	if x1 <= x0 || y1 <= y0 {
		return nil, 0, 0, fmt.Errorf("empty crop region")
	}
	sub := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	src := image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x1, b.Min.Y+y1)
	draw.Draw(sub, sub.Bounds(), img, src.Min, draw.Src)
	out, err := encodePNG(sub)
	if err != nil {
		return nil, 0, 0, err
	}
	return out, x1 - x0, y1 - y0, nil
}

// clamp constrains v to [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

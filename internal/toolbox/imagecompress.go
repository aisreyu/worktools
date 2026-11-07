package toolbox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// CompressOptions controls how an image is compressed to meet a size target.
type CompressOptions struct {
	// TargetKB is the desired maximum size in kilobytes. 0 means no size
	// target (only MaxDim/Quality apply).
	TargetKB int
	// MaxDim caps the longer edge in pixels. 0 keeps the original size.
	MaxDim int
	// Quality is the starting JPEG quality (1-100) when no size target is set.
	Quality int
	// Format is the output format: "auto" (jpeg unless alpha), "jpeg", "png" or
	// "webp".
	Format string
}

// CompressResult reports the outcome for one image.
type CompressResult struct {
	Name      string  `json:"name"`
	Mime      string  `json:"mime"`
	OrigSize  int     `json:"origSize"`
	NewSize   int     `json:"newSize"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Quality   int     `json:"quality"`
	Ratio     float64 `json:"ratio"`          // newSize / origSize
	TargetMet bool    `json:"targetMet"`      // true when TargetKB was requested and reached
	Data      string  `json:"data,omitempty"` // base64 of the compressed image
}

// CompressFile reads the image at inPath and compresses it, returning the
// result (with base64 body) plus the encoded bytes for callers that save it.
func CompressFile(inPath string, opts CompressOptions) (CompressResult, []byte, error) {
	if _, err := os.Stat(inPath); err != nil {
		return CompressResult{}, nil, fmt.Errorf("read image: %w", err)
	}
	img, err := imaging.Open(inPath, imaging.AutoOrientation(true))
	if err != nil {
		return CompressResult{}, nil, fmt.Errorf("decode image: %w", err)
	}
	origSize := 0
	if fi, err := os.Stat(inPath); err == nil {
		origSize = int(fi.Size())
	}
	return compressImage(filepath.Base(inPath), img, origSize, opts)
}

// CompressBytes compresses an image supplied as base64 (no data-URI prefix).
func CompressBytes(name, dataB64 string, opts CompressOptions) (CompressResult, []byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataB64))
	if err != nil {
		return CompressResult{}, nil, fmt.Errorf("decode base64: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return CompressResult{}, nil, fmt.Errorf("decode image: %w", err)
	}
	return compressImage(name, img, len(raw), opts)
}

// compressImage performs format selection, optional downscaling and a binary
// search over JPEG quality until the size target is met.
func compressImage(name string, img image.Image, origSize int, opts CompressOptions) (CompressResult, []byte, error) {
	if opts.MaxDim > 0 {
		b := img.Bounds()
		if b.Dx() > opts.MaxDim || b.Dy() > opts.MaxDim {
			img = imaging.Fit(img, opts.MaxDim, opts.MaxDim, imaging.Lanczos)
		}
	}

	format := resolveFormat(opts.Format, img)
	enc, mimeType, err := encodeTarget(img, format, opts)
	if err != nil {
		return CompressResult{}, nil, err
	}
	b := img.Bounds()
	res := CompressResult{
		Name:     name,
		Mime:     mimeType,
		OrigSize: origSize,
		NewSize:  len(enc.data),
		Width:    b.Dx(),
		Height:   b.Dy(),
		Quality:  enc.quality,
		Data:     base64.StdEncoding.EncodeToString(enc.data),
	}
	if origSize > 0 {
		res.Ratio = float64(res.NewSize) / float64(origSize)
	}
	if opts.TargetKB > 0 {
		res.TargetMet = res.NewSize <= opts.TargetKB*1024
	} else {
		res.TargetMet = res.Ratio < 1 || origSize == 0
	}
	return res, enc.data, nil
}

// encodedImage carries the encoded body plus the quality that produced it.
type encodedImage struct {
	data    []byte
	quality int
}

// encodeTarget encodes img to format, honouring the size target when set.
func encodeTarget(img image.Image, format string, opts CompressOptions) (encodedImage, string, error) {
	target := opts.TargetKB * 1024
	switch format {
	case "png":
		enc, err := encodePNG(img)
		if err != nil {
			return encodedImage{}, "", err
		}
		if target > 0 && len(enc) > target && opts.MaxDim == 0 {
			// PNG cannot trade quality; try a lossy pass as a fallback so the
			// size target can still be met for screenshots.
			if fb, err := jpegAtQuality(img, guessQuality(img, target)); err == nil && len(fb) < len(enc) {
				return encodedImage{data: fb, quality: -1}, "image/jpeg", nil
			}
		}
		return encodedImage{data: enc, quality: 100}, "image/png", nil
	default: // jpeg / webp fall back to jpeg encoding (widely supported)
		q := opts.Quality
		if q <= 0 {
			q = 85
		}
		if target <= 0 {
			enc, err := jpegAtQuality(img, q)
			if err != nil {
				return encodedImage{}, "", err
			}
			return encodedImage{data: enc, quality: q}, "image/jpeg", nil
		}
		data, usedQ, err := searchJPEG(img, target)
		if err != nil {
			return encodedImage{}, "", err
		}
		return encodedImage{data: data, quality: usedQ}, "image/jpeg", nil
	}
}

// searchJPEG binary-searches the JPEG quality to get under target bytes.
func searchJPEG(img image.Image, target int) ([]byte, int, error) {
	best, _ := jpegAtQuality(img, 92)
	bestQ := 0
	if len(best) <= target {
		bestQ = 92
	}
	lo, hi := 20, 90
	for lo <= hi {
		mid := (lo + hi) / 2
		enc, err := jpegAtQuality(img, mid)
		if err != nil {
			return nil, 0, err
		}
		if len(enc) <= target {
			best, bestQ = enc, mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if bestQ == 0 {
		// Even the minimum quality overshoots; return the smallest we produced.
		enc, err := jpegAtQuality(img, 20)
		if err != nil {
			return nil, 0, err
		}
		return enc, 20, nil
	}
	return best, bestQ, nil
}

// guessQuality returns a rough starting quality for a PNG fallback.
func guessQuality(img image.Image, target int) int {
	if enc, err := jpegAtQuality(img, 80); err == nil && len(enc) <= target {
		return 80
	}
	return 60
}

// jpegAtQuality encodes img as JPEG at the given quality.
func jpegAtQuality(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// encodePNG encodes img as PNG.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// CompressOutputPath derives the destination for a compressed image, appending
// a "_min" suffix and the correct extension for the output MIME type.
func CompressOutputPath(inPath, outDir, mimeType string) string {
	ext := ".jpg"
	switch mimeType {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	}
	base := strings.TrimSuffix(filepath.Base(inPath), filepath.Ext(inPath))
	name := base + "_min" + ext
	if strings.TrimSpace(outDir) == "" {
		return filepath.Join(filepath.Dir(inPath), name)
	}
	return filepath.Join(outDir, name)
}

// resolveFormat decides the output format for an image.
func resolveFormat(format string, img image.Image) string {
	switch strings.ToLower(format) {
	case "png":
		return "png"
	case "jpeg", "jpg", "webp":
		return "jpeg"
	default: // auto
		if hasAlpha(img) {
			return "png"
		}
		return "jpeg"
	}
}

// hasAlpha reports whether the image has any non-opaque pixel.
func hasAlpha(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA, *image.RGBA, *image.NRGBA64, *image.RGBA64, *image.Paletted:
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
					return true
				}
			}
		}
	}
	return false
}

package toolbox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

// ConverOptions controls image format conversion.
type ConvertOptions struct {
	// Target is the output format: jpeg, png, gif, bmp or tiff. WebP is
	// decode-only (no pure-Go encoder), so it is not a valid target.
	Target string
	// Quality is the encoder quality for lossy targets (jpeg: 1-100).
	Quality int
	// MaxDim caps the longer edge in pixels. 0 keeps the original size.
	MaxDim int
}

// ConvertResult reports the outcome for one converted image.
type ConvertResult struct {
	Name     string `json:"name"`
	FromExt  string `json:"fromExt"`
	ToFormat string `json:"toFormat"`
	Mime     string `json:"mime"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Size     int    `json:"size"`
	Data     string `json:"data,omitempty"`
}

// supportedConvertTargets are the output formats we can encode.
var supportedConvertTargets = map[string]bool{
	"jpeg": true,
	"png":  true,
	"gif":  true,
	"bmp":  true,
	"tiff": true,
}

// normalizeTarget maps common aliases to a canonical encoder name.
func normalizeTarget(target string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(target))
	switch t {
	case "jpg", "jpeg":
		return "jpeg", nil
	case "png":
		return "png", nil
	case "gif":
		return "gif", nil
	case "bmp":
		return "bmp", nil
	case "tif", "tiff":
		return "tiff", nil
	case "webp":
		return "", fmt.Errorf("webp 暂不支持编码为输出格式，仅支持作为输入")
	default:
		return "", fmt.Errorf("不支持的输出格式: %s", target)
	}
}

// ConvertFile decodes the image at inPath and re-encodes it as opts.Target,
// returning metadata (with base64 body) plus the encoded bytes.
func ConvertFile(inPath string, opts ConvertOptions) (ConvertResult, []byte, error) {
	if _, err := os.Stat(inPath); err != nil {
		return ConvertResult{}, nil, fmt.Errorf("read image: %w", err)
	}
	img, err := imaging.Open(inPath, imaging.AutoOrientation(true))
	if err != nil {
		return ConvertResult{}, nil, fmt.Errorf("decode image: %w", err)
	}
	return convertImage(filepath.Base(inPath), filepath.Ext(inPath), img, opts)
}

// ConvertBytes converts an image supplied as base64 (no data-URI prefix).
func ConvertBytes(name, dataB64 string, opts ConvertOptions) (ConvertResult, []byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataB64))
	if err != nil {
		return ConvertResult{}, nil, fmt.Errorf("decode base64: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return ConvertResult{}, nil, fmt.Errorf("decode image: %w", err)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	return convertImage(name, ext, img, opts)
}

func convertImage(name, fromExt string, img image.Image, opts ConvertOptions) (ConvertResult, []byte, error) {
	target, err := normalizeTarget(opts.Target)
	if err != nil {
		return ConvertResult{}, nil, err
	}
	if opts.MaxDim > 0 {
		b := img.Bounds()
		if b.Dx() > opts.MaxDim || b.Dy() > opts.MaxDim {
			img = imaging.Fit(img, opts.MaxDim, opts.MaxDim, imaging.Lanczos)
		}
	}
	data, mime, err := encodeTargetFormat(img, target, opts.Quality)
	if err != nil {
		return ConvertResult{}, nil, err
	}
	b := img.Bounds()
	res := ConvertResult{
		Name:     name,
		FromExt:  strings.TrimPrefix(strings.ToLower(fromExt), "."),
		ToFormat: target,
		Mime:     mime,
		Width:    b.Dx(),
		Height:   b.Dy(),
		Size:     len(data),
		Data:     base64.StdEncoding.EncodeToString(data),
	}
	return res, data, nil
}

// encodeTargetFormat encodes img using the encoder for target.
func encodeTargetFormat(img image.Image, target string, quality int) ([]byte, string, error) {
	var buf bytes.Buffer
	switch target {
	case "jpeg":
		q := quality
		if q <= 0 {
			q = 90
		}
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		return buf.Bytes(), "image/jpeg", nil
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
		return buf.Bytes(), "image/png", nil
	case "gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return nil, "", fmt.Errorf("encode gif: %w", err)
		}
		return buf.Bytes(), "image/gif", nil
	case "bmp":
		if err := bmp.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("encode bmp: %w", err)
		}
		return buf.Bytes(), "image/bmp", nil
	case "tiff":
		if err := tiff.Encode(&buf, img, nil); err != nil {
			return nil, "", fmt.Errorf("encode tiff: %w", err)
		}
		return buf.Bytes(), "image/tiff", nil
	default:
		return nil, "", fmt.Errorf("不支持的输出格式: %s", target)
	}
}

// ConvertOutputPath derives the destination for a converted image, replacing
// the extension (and appending a suffix before it).
func ConvertOutputPath(inPath, outDir, target, suffix string) string {
	target, err := normalizeTarget(target)
	if err != nil {
		target = "png"
	}
	ext := "." + target
	if target == "jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(inPath), filepath.Ext(inPath))
	name := base + suffix + ext
	if strings.TrimSpace(outDir) == "" {
		return filepath.Join(filepath.Dir(inPath), name)
	}
	return filepath.Join(outDir, name)
}

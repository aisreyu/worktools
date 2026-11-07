package toolbox

import (
	"encoding/base64"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
)

// writeTestJPEG creates a gradient JPEG of the given size on disk.
func writeTestJPEG(t *testing.T, w, h int) string {
	t.Helper()
	img := imaging.New(w, h, color.NRGBA{R: 30, G: 90, B: 200, A: 255})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	p := filepath.Join(t.TempDir(), "src.jpg")
	if err := imaging.Save(img, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	return p
}

func TestCompressMaxDim(t *testing.T) {
	p := writeTestJPEG(t, 800, 600)
	res, data, err := CompressFile(p, CompressOptions{MaxDim: 200})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.Width != 200 || res.Height != 150 {
		t.Fatalf("expected 200x150, got %dx%d", res.Width, res.Height)
	}
	if len(data) == 0 || res.NewSize != len(data) {
		t.Fatal("encoded size mismatch")
	}
	if res.Mime != "image/jpeg" {
		t.Fatalf("mime: %s", res.Mime)
	}
}

func TestCompressTargetKB(t *testing.T) {
	p := writeTestJPEG(t, 1200, 900)
	res, _, err := CompressFile(p, CompressOptions{TargetKB: 30})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.NewSize > 30*1024 {
		t.Fatalf("target not met: %d bytes", res.NewSize)
	}
	if !res.TargetMet {
		t.Fatal("TargetMet should be true")
	}
	if res.Quality < 20 || res.Quality > 92 {
		t.Fatalf("unexpected quality %d", res.Quality)
	}
}

func TestCompressBytesPNG(t *testing.T) {
	// A PNG with alpha must stay PNG under "auto".
	img := imaging.New(60, 40, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
	p := filepath.Join(t.TempDir(), "alpha.png")
	if err := imaging.Save(img, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	res, _, err := CompressBytes("alpha.png", base64.StdEncoding.EncodeToString(raw), CompressOptions{Format: "auto"})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.Mime != "image/png" {
		t.Fatalf("expected png, got %s", res.Mime)
	}
}

func TestCompressOutputPath(t *testing.T) {
	got := CompressOutputPath("/a/b/photo.JPG", "/out", "image/jpeg")
	if got != "/out/photo_min.jpg" {
		t.Fatalf("got %q", got)
	}
	if p := CompressOutputPath("/a/b/photo.png", "", "image/png"); !strings.HasSuffix(p, "photo_min.png") {
		t.Fatalf("got %q", p)
	}
}

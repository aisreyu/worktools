package toolbox

import (
	"context"
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 256), G: uint8(y % 256), B: 200, A: 255})
		}
	}
	if err := imaging.Save(img, path); err != nil {
		t.Fatalf("write test png: %v", err)
	}
}

func writeTestPDF(t *testing.T, dir string) string {
	t.Helper()
	imgPath := filepath.Join(dir, "src.png")
	writeTestPNG(t, imgPath, 300, 200)
	pdfPath := filepath.Join(dir, "src.pdf")
	conf := model.NewDefaultConfiguration()
	imp := api.DefaultImportConfig()
	if err := api.ImportImagesFile(context.Background(), []string{imgPath}, pdfPath, imp, conf); err != nil {
		t.Fatalf("import image to pdf: %v", err)
	}
	return pdfPath
}

func TestWatermarkImage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	out := filepath.Join(dir, "out.png")
	writeTestPNG(t, src, 400, 300)

	err := WatermarkImage(src, out, WatermarkTextOptions{
		Text:     "内部资料 机密",
		FontSize: 32,
		Opacity:  0.5,
		Color:    "#ff0000",
		Rotation: 30,
		Anchor:   "center",
	})
	if err != nil {
		t.Fatalf("watermark image: %v", err)
	}
	orig, _ := os.ReadFile(src)
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(got) == 0 || string(got) == string(orig) {
		t.Fatalf("output image unchanged or empty")
	}
	if b := bounds(t, out); b.Dx() != 400 || b.Dy() != 300 {
		t.Fatalf("unexpected output size %v", b)
	}
}

func TestWatermarkImageTileNonZeroAngle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	out := filepath.Join(dir, "out.png")
	writeTestPNG(t, src, 500, 400)
	if err := WatermarkImage(src, out, WatermarkTextOptions{
		Text:     "CONFIDENTIAL",
		FontSize: 24,
		Tile:     true,
		Diagonal: true,
		Opacity:  0.3,
	}); err != nil {
		t.Fatalf("watermark image (tile): %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}
}

func TestWatermarkImageRequiresText(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	writeTestPNG(t, src, 100, 100)
	if err := WatermarkImage(src, filepath.Join(dir, "o.png"), WatermarkTextOptions{Text: "  "}); err == nil {
		t.Fatalf("expected error for empty text")
	}
}

func TestRotateImage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	out := filepath.Join(dir, "out.png")
	writeTestPNG(t, src, 200, 100)
	if err := RotateImage(src, out, 90); err != nil {
		t.Fatalf("rotate image: %v", err)
	}
	b := bounds(t, out)
	if b.Dx() != 100 || b.Dy() != 200 {
		t.Fatalf("rotate 90 expected 100x200, got %v", b)
	}
}

func TestPreviewImage(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	writeTestPNG(t, src, 1200, 800)
	b64, err := PreviewImage(src, WatermarkTextOptions{Text: "预览", FontSize: 40}, 400)
	if err != nil {
		t.Fatalf("preview image: %v", err)
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode preview png: %v", err)
	}
	if img.Bounds().Dx() > 400 || img.Bounds().Dy() > 400 {
		t.Fatalf("preview not scaled: %v", img.Bounds())
	}
}

func TestWatermarkPDF(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	out := filepath.Join(dir, "out.pdf")
	err := WatermarkPDF(pdf, out, "", WatermarkTextOptions{
		Text:     "机密 内部",
		FontSize: 40,
		Opacity:  0.4,
		Rotation: 45,
		OnTop:    false,
	})
	if err != nil {
		t.Fatalf("watermark pdf: %v", err)
	}
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Fatalf("output pdf missing/empty: %v", err)
	}
}

func TestWatermarkPDFDiagonal(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	out := filepath.Join(dir, "out.pdf")
	if err := WatermarkPDF(pdf, out, "1", WatermarkTextOptions{Text: "DRAFT", Diagonal: true}); err != nil {
		t.Fatalf("watermark pdf diagonal: %v", err)
	}
}

func TestRotatePDF(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	out := filepath.Join(dir, "rot.pdf")
	if err := RotatePDF(pdf, out, 90, ""); err != nil {
		t.Fatalf("rotate pdf: %v", err)
	}
	if err := RotatePDF(pdf, out, 45, ""); err == nil {
		t.Fatalf("expected error for non-multiple-of-90 rotation")
	}
}

func TestPDFPageInfo(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	info, err := PDFPageInfo(pdf)
	if err != nil {
		t.Fatalf("page info: %v", err)
	}
	if info.Pages != 1 {
		t.Fatalf("expected 1 page, got %d", info.Pages)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Fatalf("expected positive dims, got %+v", info)
	}
}

func TestOutputPath(t *testing.T) {
	got := OutputPath("/a/b/photo.jpg", "/tmp/out", "_wm")
	want := filepath.Join("/tmp/out", "photo_wm.jpg")
	if got != want {
		t.Fatalf("OutputPath = %q, want %q", got, want)
	}
	got = OutputPath("/a/b/photo.jpg", "", "_wm")
	want = filepath.Join("/a/b", "photo_wm.jpg")
	if got != want {
		t.Fatalf("OutputPath (same dir) = %q, want %q", got, want)
	}
}

func bounds(t *testing.T, path string) image.Rectangle {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img.Bounds()
}

var _ = png.Encode


package toolbox

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

func TestWatermarkPDFTiled(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	out := filepath.Join(dir, "tiled.pdf")
	if err := WatermarkPDF(pdf, out, "", WatermarkTextOptions{
		Text:     "内部资料",
		FontSize: 30,
		Tile:     true,
		Density:  80, // 80% => gap shrinks to ~0.4 of the mark size
		Opacity:  0.3,
		Rotation: -30,
	}); err != nil {
		t.Fatalf("watermark pdf tiled: %v", err)
	}
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Fatalf("tiled output pdf missing/empty: %v", err)
	}
}

func TestDensityGapResolution(t *testing.T) {
	cases := []struct {
		name string
		pct  float64
		want float64
	}{
		{"default fifty", 0, 0.71875},
		{"twenty percent", 20, 1.0},
		{"fifty percent", 50, 0.71875},
		{"hundred percent", 100, 0.25},
		{"clamped low", 5, 1.0},
		{"clamped high", 9999, 0},
		{"one fifty percent", 150, 0.1875},
		{"two hundred percent", 200, 0.125},
		{"three hundred percent", 300, 0},
	}
	for _, tc := range cases {
		got := densityGap(WatermarkTextOptions{Density: tc.pct})
		if got != tc.want {
			t.Fatalf("%s: densityGap = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestTileLayerDensityChangesSpacing asserts the slider really tightens the
// grid: the largest blank gap shrinks as the density grows.
func TestTileLayerDensityChangesSpacing(t *testing.T) {
	face, err := imageFace(WatermarkTextOptions{Font: "sans", FontSize: 40})
	if err != nil {
		t.Fatalf("face: %v", err)
	}
	layer := renderTextLayer("内部资料", face, color.NRGBA{R: 255, A: 255})
	face.Close()

	blankAt := func(pct float64) int {
		base := renderTiledPattern(layer, 800, 600, WatermarkTextOptions{Density: pct}, 0)
		return longestBlankRun(base, 600, false)
	}
	sparse, dense, ultra := blankAt(20), blankAt(100), blankAt(300)
	if sparse <= dense {
		t.Fatalf("denser tiling should tighten the vertical gap: sparse=%d dense=%d", sparse, dense)
	}
	if dense < ultra {
		t.Fatalf("ultra density should not open the gap again: dense=%d ultra=%d", dense, ultra)
	}
	// At 300% the marks sit edge to edge, so the only blank band left is the
	// font's internal leading inside the mark's own box - never a full extra
	// mark height.
	if ultra > layer.Bounds().Dy() {
		t.Fatalf("ultra density left a %dpx blank band, mark box is %dpx", ultra, layer.Bounds().Dy())
	}
}

// TestTileLayerEvenGaps guards the layout: the vertical spacing must follow the
// mark's own aspect ratio. A square grid applied to a wide mark leaves large
// blank bands between the text rows, which showed up as "areas without a
// watermark" on the page.
func TestTileLayerEvenGaps(t *testing.T) {
	text := "内部资料"
	face, err := imageFace(WatermarkTextOptions{Font: "sans", FontSize: 40})
	if err != nil {
		t.Fatalf("face: %v", err)
	}
	layer := renderTextLayer(text, face, color.NRGBA{R: 255, A: 255})
	face.Close()

	const w, h = 800, 600
	for _, pct := range []float64{50, 100, 200, 300} {
		base := renderTiledPattern(layer, w, h, WatermarkTextOptions{Density: pct}, 0)

		// The vertical pitch must follow the mark's own aspect ratio. A
		// square grid (stepY = stepX) applied to a wide mark leaves huge blank
		// bands between the text rows, which showed up as "areas without a
		// watermark" on the page. The blank band must therefore stay well
		// below what a square grid would leave at the same density.
		blankRows := longestBlankRun(base, h, false)
		if blankRows == 0 {
			continue
		}
		gap := densityGap(WatermarkTextOptions{Density: pct})
		squareStep := float64(layer.Bounds().Dx()) * (1 + gap)
		inkH := float64(layer.Bounds().Dy()) * 0.6 // ink fills ~60% of the box
		squareBlank := squareStep - inkH
		if float64(blankRows) > squareBlank {
			t.Errorf("density %.0f%%: blank band %dpx is no better than a square grid (%.0fpx)",
				pct, blankRows, squareBlank)
		}
	}
}

// longestBlankRun returns the longest run of completely transparent lines.
func longestBlankRun(img *image.NRGBA, size int, horizontal bool) int {
	best, run := 0, 0
	for i := 0; i < size; i++ {
		ink := false
		if horizontal {
			for y := 0; y < img.Bounds().Dy() && !ink; y++ {
				ink = img.NRGBAAt(i, y).A > 0
			}
		} else {
			for x := 0; x < img.Bounds().Dx() && !ink; x++ {
				ink = img.NRGBAAt(x, i).A > 0
			}
		}
		if ink {
			run = 0
			continue
		}
		run++
		if run > best {
			best = run
		}
	}
	return best
}

// TestWatermarkPDFTiledNotRotated guards a pdfcpu pitfall: its stamp default is
// a diagonal placement, which rotates the finished pattern on the page and
// leaves triangular bare areas in the corners.
func TestWatermarkPDFTiledNotRotated(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	out := filepath.Join(dir, "tiled.pdf")
	if err := WatermarkPDF(pdf, out, "", WatermarkTextOptions{
		Text:     "内部资料",
		FontSize: 24,
		Tile:     true,
		Density:  100,
		Opacity:  0.3,
	}); err != nil {
		t.Fatalf("watermark pdf tiled: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	matrix, ok := watermarkFormMatrix(data)
	if !ok {
		t.Skip("watermark form invocation not found; pdfcpu layout changed")
	}
	if matrix != "1.00000 0.00000 0.00000 1.00000 0.00000 0.00000" {
		t.Fatalf("tiled watermark must be placed unrotated, got matrix %q", matrix)
	}
}

// pdfStreams returns the raw stream bodies of every object in pdf.
func pdfStreams(pdf []byte) [][]byte {
	var out [][]byte
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("stream"))
		if i < 0 {
			return out
		}
		rest = rest[i+len("stream"):]
		rest = bytes.TrimPrefix(rest, []byte("\r"))
		rest = bytes.TrimPrefix(rest, []byte("\n"))
		end := bytes.Index(rest, []byte("endstream"))
		if end < 0 {
			return out
		}
		out = append(out, rest[:end])
		rest = rest[end:]
	}
}

// inflate returns the flate-decoded body, or nil when it is not deflate data.
func inflate(b []byte) []byte {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}
	return data
}

// watermarkFormMatrix returns the cm matrix used to invoke the watermark form
// XObject, decompressing the page content stream when needed.
func watermarkFormMatrix(pdf []byte) (string, bool) {
	re := regexp.MustCompile(`([-\d.]+ [-\d.]+ [-\d.]+ [-\d.]+ [-\d.]+ [-\d.]+) cm /GS\d+ gs /Fm\d+ Do`)
	for _, stream := range pdfStreams(pdf) {
		for _, candidate := range [][]byte{stream, inflate(stream)} {
			if candidate == nil {
				continue
			}
			if m := re.FindSubmatch(candidate); m != nil {
				return string(m[1]), true
			}
		}
	}
	return "", false
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

func TestWatermarkImageBytes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	writeTestPNG(t, src, 320, 240)

	data, ext, err := WatermarkImageBytes(src, WatermarkTextOptions{
		Text:     "内部资料",
		FontSize: 24,
		Opacity:  0.4,
	})
	if err != nil {
		t.Fatalf("WatermarkImageBytes: %v", err)
	}
	if ext != ".png" {
		t.Fatalf("want .png, got %q", ext)
	}
	if len(data) == 0 {
		t.Fatal("empty output")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 240 {
		t.Fatalf("unexpected output size %v", b)
	}
}

func TestWatermarkImageBytesJPEG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.jpg")
	img := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	if err := imaging.Save(img, src); err != nil {
		t.Fatalf("write jpg: %v", err)
	}
	_, ext, err := WatermarkImageBytes(src, WatermarkTextOptions{Text: "X", FontSize: 16})
	if err != nil {
		t.Fatalf("WatermarkImageBytes: %v", err)
	}
	if ext != ".jpg" {
		t.Fatalf("want .jpg, got %q", ext)
	}
}

func TestWatermarkPDFBytes(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPDF(t, dir)
	data, err := WatermarkPDFBytes(src, "", WatermarkTextOptions{Text: "机密", FontSize: 24, Opacity: 0.5})
	if err != nil {
		t.Fatalf("WatermarkPDFBytes: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty pdf output")
	}
}

// Page rotation must be applied together with the watermark: the exported
// file has both the rotated page and the watermark on it.
func TestPageRotationWithWatermark(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "p.png")
	writeTestPNG(t, img, 400, 300)
	pdf := filepath.Join(dir, "doc.pdf")
	if err := api.ImportImagesFile(context.Background(), []string{img}, pdf, api.DefaultImportConfig(), model.NewDefaultConfiguration()); err != nil {
		t.Fatal(err)
	}

	// PDF: rotate 90 + watermark
	outPDF := filepath.Join(dir, "rot_wm.pdf")
	opts := WatermarkTextOptions{Text: "机密", FontSize: 40, PageRotation: 90}
	if err := WatermarkPDF(pdf, outPDF, "", opts); err != nil {
		t.Fatal(err)
	}
	dims, err := api.PageDimsFile(context.Background(), outPDF)
	if err != nil || len(dims) == 0 {
		t.Fatalf("dims: %v %v", dims, err)
	}
	// 400x300pt page rotated 90 -> 300x400
	if dims[0].Width > dims[0].Height {
		t.Errorf("page not rotated: %vx%v", dims[0].Width, dims[0].Height)
	}
	png, err := renderPDFThumbnail(outPDF, 600)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 1000 {
		t.Error("suspiciously small render")
	}
	t.Logf("rotated+watermarked PDF: %d bytes, dims %vx%v", len(png), dims[0].Width, dims[0].Height)

	// Image: rotate 90 + watermark -> portrait output
	outImg := filepath.Join(dir, "rot_wm.png")
	if err := WatermarkImage(img, outImg, WatermarkTextOptions{Text: "机密", FontSize: 40, PageRotation: 90}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(outImg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rotated+watermarked image: %d bytes", st.Size())

	// Invalid angle rejected
	if err := WatermarkImage(img, filepath.Join(dir, "x.png"), WatermarkTextOptions{Text: "机密", PageRotation: 45}); err == nil {
		t.Error("45-degree rotation should be rejected")
	}
}

// A rotated page must still show the watermark. OnTop=true because the test
// page is an opaque gradient: an underlay watermark is correctly hidden by
// the page content.
func TestRotatedPDFShowsWatermark(t *testing.T) {
	dir := t.TempDir()
	pdf := writeTestPDF(t, dir)
	out := filepath.Join(dir, "rot_wm.pdf")
	opts := WatermarkTextOptions{
		Text: "机密", FontSize: 48, Opacity: 1, OnTop: true,
		Color: "#ff0000", PageRotation: 90,
	}
	if err := WatermarkPDF(pdf, out, "", opts); err != nil {
		t.Fatal(err)
	}
	png, err := renderPDFThumbnail(out, 600)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	nrgba := imaging.Clone(img)
	b := nrgba.Bounds()
	red := 0
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := nrgba.NRGBAAt(x, y)
			if c.R > 150 && c.G < 100 && c.B < 100 {
				red++
			}
		}
	}
	if red == 0 {
		t.Fatalf("no watermark pixels visible on rotated page")
	}
	t.Logf("rotated page %dx%d shows %d red watermark px", b.Dx(), b.Dy(), red)
	_ = os.Stdout
}

var _ = png.Encode

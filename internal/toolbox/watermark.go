package toolbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcolor "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/color"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// WatermarkTextOptions describes a text watermark applied to an image or PDF.
type WatermarkTextOptions struct {
	Text     string  `json:"text"`
	FontSize float64 `json:"fontSize"` // points (PDF) or pixels (image)
	Font     string  `json:"font"`     // sans | serif | mono
	FontPath string  `json:"fontPath"` // optional explicit .ttf/.ttc file (used for images)
	Color    string  `json:"color"`    // #rrggbb (empty = gray)
	Opacity  float64 `json:"opacity"`  // 0..1
	Rotation float64 `json:"rotation"` // -180..180 degrees
	Diagonal bool    `json:"diagonal"` // true = paint along the page/image diagonal
	Tile     bool    `json:"tile"`     // true = repeat across the whole surface
	Anchor   string  `json:"anchor"`   // center|tl|tc|tr|l|r|bl|bc|br
	OnTop    bool    `json:"onTop"`    // true = stamp over content, false = watermark under
	Scale    float64 `json:"scale"`    // PDF relative scale 0..1 (0 = derive from FontSize)
}

// PDFInfo summarises a PDF file.
type PDFInfo struct {
	Pages  int     `json:"pages"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// RotateImage rotates an image by degrees (clockwise) and writes it to outPath.
func RotateImage(inPath, outPath string, degrees int) error {
	img, err := imaging.Open(inPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	rotated := imaging.Rotate(img, float64(degrees), color.White)
	if err := imaging.Save(rotated, outPath); err != nil {
		return fmt.Errorf("save image: %w", err)
	}
	return nil
}

// WatermarkImage draws a text watermark onto an image and writes it to outPath.
func WatermarkImage(inPath, outPath string, opts WatermarkTextOptions) error {
	img, err := imaging.Open(inPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	out, err := renderImageWatermark(img, opts)
	if err != nil {
		return err
	}
	if err := imaging.Save(out, outPath); err != nil {
		return fmt.Errorf("save image: %w", err)
	}
	return nil
}

// PreviewImage renders the watermarked image scaled down and returns it as
// standard base64 PNG (no data-URI prefix).
func PreviewImage(inPath string, opts WatermarkTextOptions, maxDim int) (string, error) {
	img, err := imaging.Open(inPath, imaging.AutoOrientation(true))
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	if maxDim <= 0 {
		maxDim = 900
	}
	if img.Bounds().Dx() > maxDim || img.Bounds().Dy() > maxDim {
		img = imaging.Fit(img, maxDim, maxDim, imaging.Lanczos)
	}
	out, err := renderImageWatermark(img, opts)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return "", fmt.Errorf("encode preview: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func renderImageWatermark(img image.Image, opts WatermarkTextOptions) (*image.NRGBA, error) {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return nil, fmt.Errorf("水印文字不能为空")
	}
	face, err := imageFace(opts)
	if err != nil {
		return nil, err
	}
	defer face.Close()

	fontSize := opts.FontSize
	if fontSize <= 0 {
		fontSize = 36
	}
	col := parseHexColor(opts.Color, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	col.A = clampAlpha(col.A, opts.Opacity)

	layer := renderTextLayer(text, face, col)
	angle := opts.Rotation
	if opts.Diagonal {
		angle = diagonalAngle(img.Bounds().Dx(), img.Bounds().Dy())
	}
	if angle != 0 {
		layer = imaging.Rotate(layer, angle, color.Transparent)
	}

	base := imaging.Clone(img)
	w, h := base.Bounds().Dx(), base.Bounds().Dy()

	// Wrap very wide layers so tiling stays readable.
	if opts.Tile {
		tileLayer(base, layer, w, h)
		return base, nil
	}
	cx, cy := anchorPoint(w, h, opts.Anchor, layer.Bounds().Dx(), layer.Bounds().Dy())
	drawLayer(base, layer, cx, cy)
	return base, nil
}

func tileLayer(base *image.NRGBA, layer image.Image, w, h int) {
	lw, lh := layer.Bounds().Dx(), layer.Bounds().Dy()
	gapX := lw/2 + 40
	gapY := lh/2 + 40
	stepX, stepY := lw+gapX, lh+gapY
	if stepX <= 0 || stepY <= 0 {
		drawLayer(base, layer, w/2, h/2)
		return
	}
	row := 0
	for y := stepY / 2; y < h+lh; y += stepY {
		offset := 0
		if row%2 == 1 {
			offset = stepX / 2
		}
		for x := stepX/2 - offset; x < w+lw; x += stepX {
			if x+lw/2 < 0 {
				continue
			}
			drawLayer(base, layer, x, y)
		}
		row++
	}
}

func drawLayer(base *image.NRGBA, layer image.Image, cx, cy int) {
	lw, lh := layer.Bounds().Dx(), layer.Bounds().Dy()
	r := image.Rect(cx-lw/2, cy-lh/2, cx-lw/2+lw, cy-lh/2+lh)
	draw.Draw(base, r, layer, layer.Bounds().Min, draw.Over)
}

func anchorPoint(w, h int, anchor string, lw, lh int) (int, int) {
	mx := lw/2 + 16
	my := lh/2 + 16
	a := strings.ToLower(strings.TrimSpace(anchor))
	switch a {
	case "tl":
		return mx, my
	case "tc":
		return w / 2, my
	case "tr":
		return w - mx, my
	case "l":
		return mx, h / 2
	case "r":
		return w - mx, h / 2
	case "bl":
		return mx, h - my
	case "bc":
		return w / 2, h - my
	case "br":
		return w - mx, h - my
	default:
		return w / 2, h / 2
	}
}

func diagonalAngle(w, h int) float64 {
	if w <= 0 {
		return 45
	}
	deg := -float64(h) / float64(w) * 180 / 3.141592653589793
	// pdfcpu draws bottom-left to top-right; keep within a sane visual range.
	if deg > -10 {
		deg = -30
	}
	return deg
}

func renderTextLayer(text string, face font.Face, col color.NRGBA) *image.NRGBA {
	adv := font.MeasureString(face, text).Ceil()
	m := face.Metrics()
	asc, desc := m.Ascent.Ceil(), m.Descent.Ceil()
	pad := 6
	w, h := adv+pad*2, asc+desc+pad*2
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(pad, pad+asc),
	}
	d.DrawString(text)
	return img
}

func clampAlpha(a uint8, opacity float64) uint8 {
	if opacity <= 0 {
		opacity = 1
	}
	if opacity > 1 {
		opacity = 1
	}
	return uint8(float64(a) * opacity)
}

func parseHexColor(s string, def color.NRGBA) color.NRGBA {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	if len(s) != 6 {
		return def
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8 & 0xff), B: uint8(v & 0xff), A: 255}
}

// imageFace builds a font face for drawing text onto images.
func imageFace(opts WatermarkTextOptions) (font.Face, error) {
	path := pickFontFile(opts.FontPath, opts.Font)
	if path == "" {
		return nil, fmt.Errorf("未找到可用字体，请在“字体文件”中选择一个 .ttf/.ttc 字体")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取字体失败: %w", err)
	}
	f, err := parseFontBytes(data, opts.Font)
	if err != nil {
		return nil, fmt.Errorf("解析字体失败: %w", err)
	}
	size := opts.FontSize
	if size <= 0 {
		size = 36
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("创建字体失败: %w", err)
	}
	return face, nil
}

func parseFontBytes(data []byte, family string) (*sfnt.Font, error) {
	if f, err := sfnt.Parse(data); err == nil {
		return f, nil
	}
	col, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	if col.NumFonts() <= 1 {
		return col.Font(0)
	}
	want := fontFamilyNames(family)
	best := 0
	for i := 0; i < col.NumFonts(); i++ {
		f, err := col.Font(i)
		if err != nil {
			continue
		}
		var buf sfnt.Buffer
		name, _ := f.Name(&buf, sfnt.NameIDFamily)
		lower := strings.ToLower(name)
		for _, w := range want {
			if strings.Contains(lower, w) {
				return f, nil
			}
		}
	}
	return col.Font(best)
}

func fontFamilyNames(family string) []string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "serif":
		return []string{"songti", "serif", "times"}
	case "mono":
		return []string{"mono", "courier", "menlo"}
	default:
		return []string{"pingfang", "heiti", "hiragino", "sans", "arial", "helvetica"}
	}
}

// pickFontFile returns an existing font file path. An explicit path wins;
// otherwise a platform-appropriate list of common CJK-capable fonts is probed.
func pickFontFile(explicit, family string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/STHeiti Medium.ttc",
			"/System/Library/Fonts/Supplemental/Songti.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "windows":
		candidates = []string{
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\simhei.ttf`,
			`C:\Windows\Fonts\simsun.ttc`,
			`C:\Windows\Fonts\arial.ttf`,
		}
	default:
		candidates = []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		}
	}
	// Prefer a family match before falling back to the first existing file.
	want := fontFamilyNames(family)
	fallback := ""
	for _, c := range candidates {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		if fallback == "" {
			fallback = c
		}
		lower := strings.ToLower(filepath.Base(c))
		for _, w := range want {
			if strings.Contains(lower, w) {
				return c
			}
		}
	}
	return fallback
}

// ---- PDF ----

// PDFPageInfo returns basic information about a PDF file.
func PDFPageInfo(inPath string) (PDFInfo, error) {
	count, err := api.PageCountFile(context.Background(), inPath)
	if err != nil {
		return PDFInfo{}, fmt.Errorf("读取 PDF 失败: %w", err)
	}
	info := PDFInfo{Pages: count}
	if dims, err := api.PageDimsFile(context.Background(), inPath); err == nil && len(dims) > 0 {
		info.Width = dims[0].Width
		info.Height = dims[0].Height
	}
	return info, nil
}

// WatermarkPDF adds a text watermark to the selected pages of a PDF. pages is a
// pdfcpu page selection (e.g. "1-3,5"); empty means all pages.
func WatermarkPDF(inPath, outPath, pages string, opts WatermarkTextOptions) error {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return fmt.Errorf("水印文字不能为空")
	}
	conf := model.NewDefaultConfiguration()
	wm := model.DefaultWatermarkConfig()
	wm.Mode = model.WMText
	wm.TextString = text
	wm.OnTop = opts.OnTop
	wm.FontName = pdfFontName(opts.Font)
	if hasCJK(text) {
		wm.ScriptName = "HANS"
	}
	if opts.FontSize > 0 {
		wm.FontSize = int(opts.FontSize)
	}
	if opts.Scale > 0 {
		wm.Scale = opts.Scale
		wm.ScaleAbs = false
	}
	wm.Pos = pdfAnchor(opts.Anchor)
	if opts.Opacity > 0 && opts.Opacity <= 1 {
		wm.Opacity = opts.Opacity
	}
	if col, err := pdfcolor.ParseColor("#" + strings.TrimPrefix(strings.TrimSpace(opts.Color), "#")); err == nil {
		wm.FillColor = col
		wm.StrokeColor = col
		wm.Color = col
	}
	if opts.Diagonal {
		wm.Diagonal = model.DiagonalLLToUR
		wm.Rotation = 0
	} else {
		wm.Diagonal = model.NoDiagonal
		wm.Rotation = opts.Rotation
	}
	wm.UserRotOrDiagonal = opts.Diagonal || opts.Rotation != 0

	if err := api.AddWatermarksFile(context.Background(), inPath, outPath, pageSelection(pages), wm, conf); err != nil {
		return fmt.Errorf("添加水印失败: %w", err)
	}
	return nil
}

// RotatePDF rotates the selected PDF pages by rotation degrees (multiple of 90).
func RotatePDF(inPath, outPath string, rotation int, pages string) error {
	if rotation%90 != 0 {
		return fmt.Errorf("PDF 旋转角度必须是 90 的倍数")
	}
	conf := model.NewDefaultConfiguration()
	if err := api.RotateFile(context.Background(), inPath, outPath, rotation, pageSelection(pages), conf); err != nil {
		return fmt.Errorf("旋转 PDF 失败: %w", err)
	}
	return nil
}

// PreviewPDF watermarks inPath into a temp file and renders the first page to a
// base64 PNG preview. On macOS this uses the system Quick Look renderer; other
// platforms return an error (callers should fall back to file info).
func PreviewPDF(inPath string, opts WatermarkTextOptions, maxDim int) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("当前系统不支持 PDF 预览渲染")
	}
	tmpDir, err := os.MkdirTemp("", "wm-preview")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpPDF := filepath.Join(tmpDir, "preview.pdf")
	if err := WatermarkPDF(inPath, tmpPDF, "", opts); err != nil {
		return "", err
	}
	if maxDim <= 0 {
		maxDim = 900
	}
	png, err := renderPDFThumbnail(tmpPDF, maxDim)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

func renderPDFThumbnail(pdfPath string, size int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "wm-thumb")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)
	cmd := exec.Command("qlmanage", "-t", "-s", strconv.Itoa(size), "-o", dir, pdfPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("PDF 预览渲染失败: %v", strings.TrimSpace(stderr.String()))
	}
	out := filepath.Join(dir, filepath.Base(pdfPath)+".png")
	b, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("读取预览失败: %w", err)
	}
	return b, nil
}

// OutputPath derives an output file path in outDir from inPath, appending
// suffix to the base name (e.g. "/a/b.png", "/o", "_wm" -> "/o/b_wm.png").
func OutputPath(inPath, outDir, suffix string) string {
	base := filepath.Base(inPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Dir(inPath)
	}
	return filepath.Join(outDir, name+suffix+ext)
}

func pageSelection(pages string) []string {
	pages = strings.TrimSpace(pages)
	if pages == "" {
		return nil
	}
	parts := strings.Split(pages, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func pdfFontName(family string) string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "serif":
		return "Times-Roman"
	case "mono":
		return "Courier"
	default:
		return "Helvetica"
	}
}

func pdfAnchor(anchor string) types.Anchor {
	switch strings.ToLower(strings.TrimSpace(anchor)) {
	case "tl":
		return types.TopLeft
	case "tc":
		return types.TopCenter
	case "tr":
		return types.TopRight
	case "l":
		return types.Left
	case "r":
		return types.Right
	case "bl":
		return types.BottomLeft
	case "bc":
		return types.BottomCenter
	case "br":
		return types.BottomRight
	default:
		return types.Center
	}
}

func hasCJK(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x2E80 && r <= 0x9FFF, // CJK radicals..unified
			r >= 0xF900 && r <= 0xFAFF, // compatibility
			r >= 0x3040 && r <= 0x30FF, // kana
			r >= 0xAC00 && r <= 0xD7AF: // hangul
			return true
		}
	}
	return false
}

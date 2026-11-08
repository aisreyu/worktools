package toolbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdffont "github.com/pdfcpu/pdfcpu/pkg/font"
	pdfcolor "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/color"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/aisreyu/worktools/internal/config"
)

// pdfcpu stores its configuration and installed fonts under $HOME. Android app
// processes have no $HOME, which makes pdfcpu fall back to /tmp (absent there)
// and abort. Bypassing the config file keeps every PDF operation self-contained.
func init() {
	if _, err := os.UserConfigDir(); err != nil {
		model.ConfigPath = "disable"
	}
}

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
	// Density is the tile spacing as a percentage (20..300): the higher the
	// value the tighter the tiles are packed. 0 falls back to the default 50.
	Density float64 `json:"density"`
	// PageRotation rotates the page itself (not the watermark) by a multiple
	// of 90 degrees before the watermark is applied, so preview and export
	// both show the rotated page with the watermark on it.
	PageRotation int `json:"pageRotation"`
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
	if opts.PageRotation != 0 {
		if opts.PageRotation%90 != 0 {
			return fmt.Errorf("页面旋转角度必须是 90 的倍数")
		}
		img = imaging.Rotate(img, float64(opts.PageRotation), color.White)
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
	if opts.PageRotation != 0 {
		if opts.PageRotation%90 != 0 {
			return "", fmt.Errorf("页面旋转角度必须是 90 的倍数")
		}
		img = imaging.Rotate(img, float64(opts.PageRotation), color.White)
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

	w, h := img.Bounds().Dx(), img.Bounds().Dy()

	if opts.Tile {
		// Tile first, then rotate the whole pattern: rotating the mark up front
		// leaves the corners of its bounding box empty, which shows up as
		// diagonal blank bands once the tiles are placed side by side.
		base := imaging.Clone(img)
		pat := renderTiledPattern(layer, w, h, opts, angle)
		draw.Draw(base, base.Bounds(), pat, pat.Bounds().Min, draw.Over)
		return base, nil
	}

	if angle != 0 {
		layer = imaging.Rotate(layer, angle, color.Transparent)
	}
	base := imaging.Clone(img)
	cx, cy := anchorPoint(w, h, opts.Anchor, layer.Bounds().Dx(), layer.Bounds().Dy())
	drawLayer(base, layer, cx, cy)
	return base, nil
}

// Density range accepted by the toolbox (percentage, higher = tighter).
const (
	densityMinPct     = 20
	densityMaxPct     = 300
	densityDefaultPct = 50
)

// densityGap converts the Density percentage into the gap between
// neighbouring marks, expressed as a fraction of the mark's own size. The
// mark itself is never resized: the font size fixes how big each mark is and
// the density only tightens the spacing, so "denser" always means "more
// marks", never "smaller text". 20% leaves a full-size gap, 100% a
// quarter-size gap and 300% packs the marks edge to edge. 0 falls back to the
// default 50%.
func densityGap(opts WatermarkTextOptions) float64 {
	pct := opts.Density
	if pct <= 0 {
		pct = densityDefaultPct
	}
	if pct < densityMinPct {
		pct = densityMinPct
	}
	if pct > densityMaxPct {
		pct = densityMaxPct
	}
	if pct <= 100 {
		return 1.0 - (pct-20.0)/80.0*0.75
	}
	return 0.25 * (1.0 - (pct-100.0)/200.0)
}

// renderTiledPattern returns a w x h transparent canvas holding the mark
// repeated on a grid derived from the density, rotated as a whole when angle is
// non-zero. Rotating after tiling keeps the spacing uniform in every direction.
func renderTiledPattern(layer image.Image, w, h int, opts WatermarkTextOptions, angle float64) *image.NRGBA {
	lw, lh := float64(layer.Bounds().Dx()), float64(layer.Bounds().Dy())
	if lw <= 0 || lh <= 0 {
		out := image.NewNRGBA(image.Rect(0, 0, w, h))
		drawLayer(out, layer, w/2, h/2)
		return out
	}

	// The mark is never resized: the font size fixes its size and the density
	// only controls the gap between tiles (a zero gap packs them edge to
	// edge, which is the densest layout possible at this font size).
	gap := densityGap(opts)
	stepX := int(math.Round(lw * (1 + gap)))
	if stepX < 1 {
		stepX = 1
	}

	// The vertical pitch follows the mark's own aspect ratio instead of the
	// page width. A square grid leaves wide blank bands between the text rows
	// because a mark is far wider than it is tall.
	stepY := int(math.Round(float64(stepX) * lh / lw))
	if stepY < 1 {
		stepY = 1
	}

	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	if angle == 0 {
		tileInto(out, layer, stepX, stepY)
		return out
	}
	// Rotate the mark once and place the copies on a lattice rotated by the
	// same angle, which is equivalent to rotating a fully tiled canvas but
	// without allocating an oversized intermediate image.
	rot := imaging.Rotate(layer, angle, color.Transparent)
	tileIntoRotated(out, rot, stepX, stepY, angle)
	return out
}

// tileIntoRotated repeats an already-rotated layer on a lattice rotated by the
// same angle. Rotating the lattice with the mark keeps the gaps uniform; a
// plain grid of rotated marks leaves diagonal blank bands, because a rotated
// mark only fills a diagonal slice of its bounding box.
func tileIntoRotated(canvas *image.NRGBA, layer image.Image, stepX, stepY int, angle float64) {
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}
	lw, lh := layer.Bounds().Dx(), layer.Bounds().Dy()
	b := canvas.Bounds()
	cx := float64(b.Min.X) + float64(b.Dx())/2
	cy := float64(b.Min.Y) + float64(b.Dy())/2

	theta := angle * math.Pi / 180
	cos, sin := math.Cos(theta), math.Sin(theta)
	// Lattice basis: one step along the text row, one step along the row stack.
	ax, ay := float64(stepX)*cos, float64(stepX)*sin
	bx, by := -float64(stepY)*sin, float64(stepY)*cos

	// Cover the canvas diagonal plus the mark so no edge is left bare.
	radius := math.Hypot(float64(b.Dx()), float64(b.Dy()))/2 + math.Max(float64(lw), float64(lh))
	ni := int(radius/float64(stepX)) + 2
	nj := int(radius/float64(stepY)) + 2

	for j := -nj; j <= nj; j++ {
		// Stagger every other row by half a step so the marks do not line up.
		off := 0.0
		if j%2 != 0 {
			off = 0.5
		}
		for i := -ni; i <= ni; i++ {
			x := cx + (float64(i)+off)*ax + float64(j)*bx
			y := cy + (float64(i)+off)*ay + float64(j)*by
			drawLayer(canvas, layer, int(math.Round(x)), int(math.Round(y)))
		}
	}
}

// tileInto repeats layer across canvas on a stepX x stepY grid.
func tileInto(canvas *image.NRGBA, layer image.Image, stepX, stepY int) {
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}
	lw, lh := layer.Bounds().Dx(), layer.Bounds().Dy()
	b := canvas.Bounds()
	// Stagger every other row by half a pitch so the marks do not line up into
	// distracting vertical stripes.
	row := 0
	for y := b.Min.Y + stepY/2; y < b.Max.Y+lh; y += stepY {
		offset := 0
		if row%2 == 1 {
			offset = stepX / 2
		}
		for x := b.Min.X + stepX/2 - offset; x < b.Max.X+lw; x += stepX {
			if x+lw/2 < b.Min.X {
				continue
			}
			drawLayer(canvas, layer, x, y)
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

// fontCandidates lists well-known CJK-capable font files per platform.
func fontCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/STHeiti Medium.ttc",
			"/System/Library/Fonts/Supplemental/Songti.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "windows":
		return []string{
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\simhei.ttf`,
			`C:\Windows\Fonts\simsun.ttc`,
			`C:\Windows\Fonts\arial.ttf`,
		}
	case "android":
		// Android (and HarmonyOS, which is built on it) keeps its fonts here.
		return []string{
			"/system/fonts/NotoSansCJK-Regular.ttc",
			"/system/fonts/NotoSerifCJK-Regular.ttc",
			"/system/fonts/NotoSansSC-Regular.otf",
			"/system/fonts/NotoSansTC-Regular.otf",
			"/system/fonts/DroidSansFallback.ttf",
			"/product/fonts/NotoSansCJK-Regular.ttc",
		}
	default:
		return []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		}
	}
}

// fontDirs lists the directories that hold system fonts per platform.
func fontDirs() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/System/Library/Fonts", "/System/Library/Fonts/Supplemental", "/Library/Fonts"}
	case "windows":
		return []string{`C:\Windows\Fonts`}
	case "android":
		return []string{"/system/fonts", "/product/fonts"}
	default:
		return []string{"/usr/share/fonts", "/usr/local/share/fonts"}
	}
}

// cjkProbeRune is used to confirm a font can actually draw Chinese text.
const cjkProbeRune = '中'

// fontNameHints orders scanned font files by how likely they hold CJK glyphs.
var fontNameHints = []string{
	"notosanscjk", "notosanssc", "notosanstc", "sourcehansans",
	"droidsansfallback", "harmonyos", "pingfang", "hiragino", "heiti",
	"songti", "wqy", "msyh", "simhei", "simsun",
}

// maxScanFonts caps how many font files are opened while looking for a CJK face.
const maxScanFonts = 40

func fontNameRank(name string) int {
	lower := strings.ToLower(name)
	for i, hint := range fontNameHints {
		if strings.Contains(lower, hint) {
			return i
		}
	}
	return len(fontNameHints)
}

var (
	systemFontOnce sync.Once
	systemFontFile string
)

// systemFont resolves (once) a CJK-capable font installed by the OS. Fixed
// paths cover desktop platforms; Android and HarmonyOS name their fonts per
// OEM and OS version, so those font directories are scanned instead.
func systemFont() string {
	systemFontOnce.Do(func() {
		for _, p := range fontCandidates() {
			if _, err := os.Stat(p); err == nil {
				systemFontFile = p
				return
			}
		}
		systemFontFile = scanFontDirs()
	})
	return systemFontFile
}

// scanFontDirs returns the best font file found under fontDirs(): hinted names
// first, then anything else that can render the probe rune.
func scanFontDirs() string {
	var files []string
	for _, dir := range fontDirs() {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".ttc", ".otf":
				files = append(files, path)
			}
			return nil
		})
	}
	if len(files) == 0 {
		return ""
	}
	sort.SliceStable(files, func(i, j int) bool {
		ri, rj := fontNameRank(filepath.Base(files[i])), fontNameRank(filepath.Base(files[j]))
		if ri != rj {
			return ri < rj
		}
		return files[i] < files[j]
	})
	if len(files) > maxScanFonts {
		files = files[:maxScanFonts]
	}
	for _, p := range files {
		if fontHasGlyph(p, cjkProbeRune) {
			return p
		}
	}
	return ""
}

// fontHasGlyph reports whether the font file can render r.
func fontHasGlyph(path string, r rune) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	f, err := parseFontBytes(data, "")
	if err != nil {
		return false
	}
	var buf sfnt.Buffer
	idx, err := f.GlyphIndex(&buf, r)
	return err == nil && idx != 0
}

// pickFontFile returns an existing font file path. An explicit path wins; then
// a platform-appropriate list of common CJK-capable fonts is probed, preferring
// one that matches the requested family.
func pickFontFile(explicit, family string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	want := fontFamilyNames(family)
	for _, c := range fontCandidates() {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		lower := strings.ToLower(filepath.Base(c))
		for _, w := range want {
			if strings.Contains(lower, w) {
				return c
			}
		}
	}
	return systemFont()
}

// ---- PDF ----

// pdfCJKFontCandidates lists CJK-capable TrueType fonts probed per platform.
func pdfCJKFontCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/STHeiti Medium.ttc",
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Supplemental/Songti.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "windows":
		return []string{
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\simhei.ttf`,
			`C:\Windows\Fonts\simsun.ttc`,
		}
	case "android":
		return append(fontCandidates(), systemFont())
	default:
		return []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		}
	}
}

var (
	pdfCJKFontOnce sync.Once
	pdfCJKFontName string
)

// pdfCJKFont installs and returns the PostScript name of an embedded CJK font
// for use with pdfcpu (once per process). It returns "" when no CJK-capable
// font is available, in which case callers fall back to a core font.
func pdfCJKFont() string {
	pdfCJKFontOnce.Do(func() {
		ctx := context.Background()
		for _, path := range pdfCJKFontCandidates() {
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if name, err := installCJKFontForPDF(ctx, path); err == nil && name != "" {
				pdfCJKFontName = name
				return
			}
		}
	})
	return pdfCJKFontName
}

// pdfFontDir returns a writable directory that holds pdfcpu's installed fonts.
func pdfFontDir() string {
	dir, err := config.AppDir()
	if err != nil {
		return os.TempDir()
	}
	fontDir := filepath.Join(dir, "pdf-fonts")
	if err := os.MkdirAll(fontDir, 0o700); err != nil {
		return os.TempDir()
	}
	return fontDir
}

// pdfConfig returns a pdfcpu configuration whose font lookups resolve against
// the app's own writable font directory instead of the config/pdfcpu one.
func pdfConfig() *model.Configuration {
	conf := model.NewDefaultConfiguration()
	pdffont.UserFontDir = pdfFontDir()
	return conf
}

// installCJKFontForPDF makes a font available to pdfcpu and returns its
// PostScript name. TrueType collections (.ttc) contain several faces; each is
// installed so its PostScript name can be resolved.
func installCJKFontForPDF(ctx context.Context, path string) (string, error) {
	dir := pdfFontDir()
	pdffont.UserFontDir = dir
	repo := pdffont.RepositoryForDir(dir)
	if strings.EqualFold(filepath.Ext(path), ".ttc") {
		rep, err := pdffont.InstallTrueTypeCollection(dir, path)
		if err != nil {
			return "", err
		}
		for _, f := range rep.Fonts {
			if ok, err := repo.IsUserFont(ctx, f.PostScriptName); err == nil && ok {
				return f.PostScriptName, nil
			}
		}
		return "", fmt.Errorf("字体集合 %s 中未找到可用字体", filepath.Base(path))
	}
	rep, err := pdffont.InstallTrueTypeFont(dir, path)
	if err != nil {
		return "", err
	}
	if len(rep.Fonts) == 0 {
		return "", fmt.Errorf("字体 %s 安装失败", filepath.Base(path))
	}
	return rep.Fonts[0].PostScriptName, nil
}

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

// tileRenderDPI is the supersample factor used when rasterising a tiled PDF
// watermark; the pattern is rendered at this multiple of 72 dpi so the stamped
// text stays crisp when printed.
const tileRenderDPI = 2.0

// buildTiledPagePNG renders the watermark repeated across a page-sized
// transparent canvas. Transparency becomes the PDF soft mask, so only the marks
// are visible once stamped. pageW/pageH are PDF points.
func buildTiledPagePNG(opts WatermarkTextOptions, pageW, pageH float64) ([]byte, error) {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return nil, fmt.Errorf("水印文字不能为空")
	}
	s := tileRenderDPI
	face, err := imageFace(WatermarkTextOptions{Font: opts.Font, FontPath: opts.FontPath, FontSize: opts.FontSize * s})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	col := parseHexColor(opts.Color, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	col.A = clampAlpha(col.A, opts.Opacity)

	layer := renderTextLayer(text, face, col)
	w := int(math.Ceil(pageW * s))
	h := int(math.Ceil(pageH * s))
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("页面尺寸无效")
	}
	angle := opts.Rotation
	if opts.Diagonal {
		angle = diagonalAngle(w, h)
	}

	canvas := renderTiledPattern(layer, w, h, opts, angle)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("encode tile: %w", err)
	}
	return buf.Bytes(), nil
}

// WatermarkPDFTiled stamps a tiled text watermark onto the selected pages by
// rendering the pattern onto a page-sized transparent image and stamping it as
// a full-page (soft-masked) watermark.
func WatermarkPDFTiled(inPath, outPath, pages string, opts WatermarkTextOptions) error {
	dims, err := api.PageDimsFile(context.Background(), inPath)
	if err != nil {
		return fmt.Errorf("读取 PDF 失败: %w", err)
	}
	if len(dims) == 0 {
		return fmt.Errorf("PDF 没有可用页面")
	}
	pngBytes, err := buildTiledPagePNG(opts, dims[0].Width, dims[0].Height)
	if err != nil {
		return err
	}
	// The third argument is a pdfcpu watermark description, not a name: a
	// non-empty value must be valid config syntax or parsing fails. Configure
	// the returned Watermark programmatically instead.
	wm, err := api.ImageWatermarkForReader(context.Background(), bytes.NewReader(pngBytes), "", opts.OnTop, true, types.POINTS)
	if err != nil {
		return fmt.Errorf("构建平铺水印失败: %w", err)
	}
	// Relative scale 1.0 fits the pattern to each page (so mixed page sizes
	// still tile edge to edge), and transparency keeps it clean.
	wm.Pos = types.Center
	wm.Scale = 1.0
	wm.ScaleAbs = false

	// The rotation is already baked into the pattern. pdfcpu defaults to a
	// diagonal stamp, which rotates the finished pattern on the page and leaves
	// the corners bare - the "missing watermark" areas.
	wm.Diagonal = model.NoDiagonal
	wm.Rotation = 0
	wm.UserRotOrDiagonal = false

	conf := pdfConfig()
	if err := api.AddWatermarksFile(context.Background(), inPath, outPath, pageSelection(pages), wm, conf); err != nil {
		return fmt.Errorf("添加水印失败: %w", err)
	}
	return nil
}

// WatermarkPDF adds a text watermark to the selected pages of a PDF. pages is a
// pdfcpu page selection (e.g. "1-3,5"); empty means all pages.
func WatermarkPDF(inPath, outPath, pages string, opts WatermarkTextOptions) error {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return fmt.Errorf("水印文字不能为空")
	}
	if opts.PageRotation != 0 {
		// Rotate the page first so the watermark lands on the rotated page
		// (matching what the preview shows), then stamp on top.
		if opts.PageRotation%90 != 0 {
			return fmt.Errorf("页面旋转角度必须是 90 的倍数")
		}
		tmpDir, err := os.MkdirTemp("", "wm-rotate")
		if err != nil {
			return fmt.Errorf("创建临时目录失败: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		rotated := filepath.Join(tmpDir, "rotated.pdf")
		if err := RotatePDF(inPath, rotated, opts.PageRotation, pages); err != nil {
			return err
		}
		inPath = rotated
	}
	if opts.Tile {
		return WatermarkPDFTiled(inPath, outPath, pages, opts)
	}
	conf := pdfConfig()
	wm := model.DefaultWatermarkConfig()
	wm.Mode = model.WMText
	wm.TextString = text
	wm.OnTop = opts.OnTop
	wm.FontName = pdfFontName(opts.Font)
	if hasCJK(text) {
		// The 14 core PDF fonts have no CJK glyphs, so Chinese/Japanese/Korean
		// text would render as invisible .notdef glyphs. pdfcpu needs an
		// installed user TrueType font for CJK; if none is available we fall
		// back to the core font (which at least renders any ASCII in the text).
		if name := pdfCJKFont(); name != "" {
			wm.FontName = name
			wm.ScriptName = "HANS"
		}
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
	conf := pdfConfig()
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

// WatermarkImageBytes renders a watermarked image and returns the encoded bytes
// plus the file extension (including the dot) chosen for that encoding.
func WatermarkImageBytes(inPath string, opts WatermarkTextOptions) ([]byte, string, error) {
	img, err := imaging.Open(inPath, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("open image: %w", err)
	}
	if opts.PageRotation != 0 {
		if opts.PageRotation%90 != 0 {
			return nil, "", fmt.Errorf("页面旋转角度必须是 90 的倍数")
		}
		img = imaging.Rotate(img, float64(opts.PageRotation), color.White)
	}
	out, err := renderImageWatermark(img, opts)
	if err != nil {
		return nil, "", err
	}
	ext := strings.ToLower(filepath.Ext(inPath))
	if ext != ".png" {
		ext = ".jpg"
	}
	var buf bytes.Buffer
	if ext == ".png" {
		if err := png.Encode(&buf, out); err != nil {
			return nil, "", fmt.Errorf("encode image: %w", err)
		}
	} else {
		if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 92}); err != nil {
			return nil, "", fmt.Errorf("encode image: %w", err)
		}
	}
	return buf.Bytes(), ext, nil
}

// WatermarkPDFBytes watermarks a PDF and returns the result as bytes.
func WatermarkPDFBytes(inPath, pages string, opts WatermarkTextOptions) ([]byte, error) {
	dir, err := os.MkdirTemp("", "wm-out")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "out.pdf")
	if err := WatermarkPDF(inPath, tmp, pages, opts); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(tmp)
	if err != nil {
		return nil, fmt.Errorf("读取水印结果失败: %w", err)
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

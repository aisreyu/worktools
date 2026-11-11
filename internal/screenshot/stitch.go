package screenshot

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
)

// StitchMode selects how consecutive frames are joined.
const (
	// StitchModeScroll overlaps-detects a vertical strip to join on.
	StitchModeScroll = "scroll"
	// StitchModeAppend simply stacks frames top to bottom (manual fallback).
	StitchModeAppend = "append"
)

// StitchResult is the outcome of a stitch operation.
type StitchResult struct {
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Frames  int      `json:"frames"`
	Mode    string   `json:"mode"`
	Overlaps []int   `json:"overlaps"` // rows trimmed from each frame after the first
	Data    string   `json:"data"`     // base64 PNG
}

// Stitch joins captured frames into a single tall image.
//
// In scroll mode it searches for the vertical overlap between each consecutive
// pair by matching a band of rows from the top of the next frame against the
// bottom of the accumulated image, then trims the duplicate rows. It falls back
// to a plain append when no confident overlap is found, so a stitch always
// produces a usable image.
func Stitch(frames []Shot, mode string) (StitchResult, error) {
	if len(frames) == 0 {
		return StitchResult{}, fmt.Errorf("no frames to stitch")
	}
	imgs := make([]image.Image, 0, len(frames))
	for i, f := range frames {
		img, _, err := image.Decode(bytes.NewReader(f.Data))
		if err != nil {
			return StitchResult{}, fmt.Errorf("decode frame %d: %w", i, err)
		}
		imgs = append(imgs, img)
	}

	base := imgs[0]
	width := base.Bounds().Dx()
	overlaps := make([]int, 0, len(imgs)-1)

	// If widths differ, normalise to the narrowest so the canvas is rectangular.
	for _, img := range imgs {
		if img.Bounds().Dx() < width {
			width = img.Bounds().Dx()
		}
	}

	result := image.NewRGBA(image.Rect(0, 0, width, 0))
	draw.Draw(result, base.Bounds(), base, base.Bounds().Min, draw.Src)
	height := base.Bounds().Dy()

	for i := 1; i < len(imgs); i++ {
		next := imgs[i]
		trim := 0
		if mode == StitchModeScroll {
			trim = detectOverlap(result, height, width, next)
		}
		overlaps = append(overlaps, trim)

		visible := next.Bounds().Dy() - trim
		if visible <= 0 {
			continue
		}
		grown := image.NewRGBA(image.Rect(0, 0, width, height+visible))
		draw.Draw(grown, result.Bounds(), result, image.Point{}, draw.Src)
		src := image.Rect(0, trim, width, next.Bounds().Dy())
		dst := image.Rect(0, height, width, height+visible)
		draw.Draw(grown, dst, next, src.Min, draw.Src)
		result = grown
		height += visible
	}

	png, err := encodePNG(result)
	if err != nil {
		return StitchResult{}, err
	}
	return StitchResult{
		Width:    width,
		Height:   height,
		Frames:   len(imgs),
		Mode:     mode,
		Overlaps: overlaps,
		Data:     base64PNG(png),
	}, nil
}

// detectOverlap looks for how many rows at the top of next are already present
// at the bottom of canvas (a height x width RGBA image). It returns the number
// of rows to trim (0 when no confident overlap is found). The search uses a
// downscaled grayscale comparison for speed and robustness.
func detectOverlap(canvas *image.RGBA, height, width int, next image.Image) int {
	const maxSearch = 1600 // cap the search band (in source rows)
	const minOverlap = 24  // ignore trivially small overlaps
	const sampleStep = 3   // px; sparse sampling keeps this fast

	search := height
	if search > maxSearch {
		search = maxSearch
	}
	nh := next.Bounds().Dy()
	if search > nh {
		search = nh
	}
	if search <= minOverlap {
		return 0
	}

	// smallGray converts an image region to a coarse grayscale row signature.
	rowSig := func(img image.Image, y, x0, step, w int) []int {
		sig := make([]int, 0, w/step+1)
		for x := x0; x < x0+w; x += step {
			r, g, b, _ := img.At(x, y).RGBA()
			sig = append(sig, int((r+g+b)/3)>>8)
		}
		return sig
	}

	best := 0
	bestScore := 1 << 30
	for overlap := search; overlap >= minOverlap; overlap-- {
		// Compare the last `overlap` rows of both: bottom of canvas vs top of next.
		sampleN := sampleRows(overlap, 12)
		score := 0
		for k := 0; k < sampleN; k++ {
			off := 0
			if sampleN > 1 {
				off = k * (overlap - 1) / (sampleN - 1)
			}
			cy := height - overlap + off
			ny := off
			cs := rowSig(canvas, cy, 0, sampleStep, width)
			ns := rowSig(next, ny, 0, sampleStep, width)
			score += diffRow(cs, ns)
		}
		avg := score / sampleN
		if avg < bestScore {
			bestScore = avg
			best = overlap
		}
	}
	// Only accept the result when the match is genuinely close.
	if bestScore <= 8 {
		return best
	}
	return 0
}

// sampleRows picks how many rows to sample across an overlap band.
func sampleRows(overlap, want int) int {
	if overlap < want {
		return overlap
	}
	return want
}

// diffRow returns the average absolute difference between two row signatures.
func diffRow(a, b []int) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 1 << 30
	}
	sum := 0
	for i := 0; i < n; i++ {
		d := a[i] - b[i]
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return sum / n
}

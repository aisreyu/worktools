package screenshot

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OCRResult is the recognised text plus the engine that produced it.
type OCRResult struct {
	Text   string `json:"text"`
	Engine string `json:"engine"`
	Lang   string `json:"lang"`
	Lines  int    `json:"lines"`
	Chars  int    `json:"chars"`
}

// OCREngineAvailable reports whether the local tesseract binary is installed.
func OCREngineAvailable() bool {
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// OCRImage runs OCR over PNG bytes using the local tesseract binary.
//
// lang is a tesseract language code ("eng", "chi_sim", ...). When empty it
// defaults to "eng". The "--psm 6" page-segmentation mode treats the image as a
// single uniform block of text, which suits screenshots of UI/lines well.
func OCRImage(data []byte, lang string) (OCRResult, error) {
	if len(data) == 0 {
		return OCRResult{}, fmt.Errorf("no image data")
	}
	if !OCREngineAvailable() {
		return OCRResult{}, fmt.Errorf("tesseract is not installed; install it to use OCR")
	}
	lang = strings.TrimSpace(lang)
	if lang == "" {
		lang = "eng"
	}

	dir, err := os.MkdirTemp("", "wt-ocr-*")
	if err != nil {
		return OCRResult{}, err
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "in.png")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return OCRResult{}, err
	}
	// Ask tesseract to write the text to a file so we never rely on stdout
	// encoding quirks ("-c" not needed; base output name, no extension).
	outBase := filepath.Join(dir, "out")

	args := []string{in, outBase, "-l", lang, "--psm", "6"}
	cmd := exec.Command("tesseract", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return OCRResult{}, fmt.Errorf("tesseract failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	txtPath := outBase + ".txt"
	raw, err := os.ReadFile(txtPath)
	if err != nil {
		return OCRResult{}, fmt.Errorf("read OCR output: %w", err)
	}
	text := normaliseOCRText(string(raw))
	return OCRResult{
		Text:   text,
		Engine: "tesseract",
		Lang:   lang,
		Lines:  countLines(text),
		Chars:  len([]rune(text)),
	}, nil
}

// normaliseOCRText trims trailing spaces on each line and collapses runs of
// blank lines, which tesseract tends to over-produce.
func normaliseOCRText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, ln := range lines {
		ln = strings.TrimRight(ln, " \t")
		if strings.TrimSpace(ln) == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// countLines counts non-empty lines in a text block.
func countLines(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

// OCREngineInfo describes OCR availability for the UI.
func OCREngineInfo() map[string]any {
	available := OCREngineAvailable()
	version := ""
	var langs []string
	if available {
		if out, err := exec.Command("tesseract", "--version").Output(); err == nil {
			line := strings.SplitN(string(out), "\n", 2)[0]
			version = strings.TrimSpace(line)
		}
		if out, err := exec.Command("tesseract", "--list-langs").Output(); err == nil {
			for _, ln := range strings.Split(string(out), "\n") {
				ln = strings.TrimSpace(ln)
				if ln == "" || strings.HasPrefix(strings.ToLower(ln), "list of available") {
					continue
				}
				langs = append(langs, ln)
			}
		}
	}
	return map[string]any{
		"available": available,
		"version":   version,
		"langs":     langs,
		"platform":  runtime.GOOS,
	}
}

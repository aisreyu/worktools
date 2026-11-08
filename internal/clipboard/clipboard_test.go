package clipboard

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := Normalize("a\r\nb\r\n"); got != "a\nb" {
		t.Errorf("Normalize = %q, want %q", got, "a\nb")
	}
	if got := Normalize("x  "); got != "x" {
		t.Errorf("Normalize trailing = %q, want %q", got, "x")
	}
}

func TestDetectKind(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"https://example.com/a?b=1", KindLink},
		{"www.example.com", KindLink},
		{"someone@example.com", KindLink},
		{"hello world", KindText},
		{"func main() {\n\tprintln(\"hi\")\n}", KindCode},
		{"#!/bin/bash\nset -e\necho hi", KindCode},
		{"a\nb\nc", KindCode},
	}
	for _, c := range cases {
		if got := DetectKind(c.text); got != c.want {
			t.Errorf("DetectKind(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestPreview(t *testing.T) {
	if got := Preview("one\n\ntwo   three"); got != "one two three" {
		t.Errorf("Preview flatten = %q", got)
	}
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	got := Preview(long)
	if len([]rune(got)) != PreviewRunes+1 { // +1 for the ellipsis
		t.Errorf("Preview length = %d, want %d", len([]rune(got)), PreviewRunes+1)
	}
}

func TestDetectKindPaths(t *testing.T) {
	if got := DetectKind("/Users/me/Documents/report.pdf"); got != KindFile {
		t.Errorf("absolute path kind = %q, want %q", got, KindFile)
	}
	if got := DetectKind("~/Downloads/a.zip\n/tmp/b.txt"); got != KindFile {
		t.Errorf("multi-path kind = %q, want %q", got, KindFile)
	}
	if got := DetectKind("/tmp/with space.txt"); got == KindFile {
		t.Errorf("path with space should not classify as file")
	}
}

func TestFileCapturePreview(t *testing.T) {
	c := FileCapture([]string{"/a/b/one.txt", "/c/two.txt"})
	if c.Kind != KindFile {
		t.Fatalf("kind = %q, want %q", c.Kind, KindFile)
	}
	if p := PreviewFor(c.Kind, c.Text); p != "one.txt (+1)" {
		t.Errorf("preview = %q, want %q", p, "one.txt (+1)")
	}
	if empty := FileCapture(nil); empty.Text != "" {
		t.Errorf("empty FileCapture text = %q, want empty", empty.Text)
	}
}

func TestImageCapture(t *testing.T) {
	// 1x1 PNG header: signature + IHDR length/type + width/height.
	png := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02,
	}
	c := ImageCapture(png)
	if c.Kind != KindImage || c.Mime != "image/png" {
		t.Fatalf("kind/mime = %q/%q", c.Kind, c.Mime)
	}
	if w, h := pngSize(png); w != 1 || h != 2 {
		t.Errorf("pngSize = %d,%d, want 1,2", w, h)
	}
	if p := PreviewFor(c.Kind, c.Text); p == "" || p[0] != 'I' {
		t.Errorf("image preview = %q", p)
	}
	if _, after, ok := strings.Cut(c.Text, "\n"); !ok || after == "" {
		t.Errorf("image text should include a hash line, got %q", c.Text)
	}
}

package clipboard

import "testing"

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

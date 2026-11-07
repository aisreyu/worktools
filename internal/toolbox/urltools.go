package toolbox

import (
	"net/url"
	"strings"
)

// URLEncodeComponent percent-encodes a string for embedding inside a URL.
// Mode "path" preserves '/' separators (escaping each path segment), anything
// else uses query escaping (spaces become '+'), matching how params appear in
// the address bar.
func URLEncodeComponent(s, mode string) string {
	if mode == "path" {
		return (&url.URL{Path: s}).EscapedPath()
	}
	return url.QueryEscape(s)
}

// URLDecodeComponent reverses percent-encoding previously applied to a URL
// component. It is tolerant of input that is not actually encoded.
func URLDecodeComponent(s, mode string) (string, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "%") && !strings.Contains(s, "+") {
		return s, nil
	}
	if mode == "path" {
		return url.PathUnescape(s)
	}
	return url.QueryUnescape(s)
}

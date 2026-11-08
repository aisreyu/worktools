// Package clipboard holds the pure-text helpers shared by the clipboard
// manager: normalising captures, classifying their kind and building previews.
// Storage and encryption live in the vault; this package never touches disk.
package clipboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Kinds of capture the manager recognises.
const (
	KindText  = "text"  // free-form text
	KindLink  = "link"  // a URL or email address
	KindCode  = "code"  // source code / structured snippets
	KindImage = "image" // an image (screenshot, copied picture) stored as PNG
	KindFile  = "file"  // one or more filesystem paths copied from a file manager
)

// pngSize reads the pixel width and height from a PNG stream header. It returns
// (0, 0) when the payload is not a decodable PNG; callers should tolerate that.
func pngSize(png []byte) (int, int) {
	// 8-byte signature + 4-byte length + 4-byte "IHDR" + 8 bytes of dimensions.
	if len(png) < 24 || !bytes.Equal(png[:8], pngSignature) || string(png[12:16]) != "IHDR" {
		return 0, 0
	}
	w := int(binary.BigEndian.Uint32(png[16:20]))
	h := int(binary.BigEndian.Uint32(png[20:24]))
	return w, h
}

// PreviewRunes caps how many characters a preview keeps before eliding.
const PreviewRunes = 80

// MaxImageBytes caps how large an image capture may be before it is skipped.
const MaxImageBytes = 8 << 20 // 8 MiB

// Capture is a single clipboard reading: the kind, its copyable text form and,
// for images, the raw PNG payload.
type Capture struct {
	Kind string
	Text string
	Data []byte // PNG bytes; only set for KindImage
	Mime string
}

// Key returns a stable identity for a capture, used to skip consecutive reads
// that have not changed. It hashes image payloads and uses the raw text
// otherwise.
func (c Capture) Key() string {
	if c.Kind == "" {
		return ""
	}
	if len(c.Data) > 0 {
		sum := sha256.Sum256(c.Data)
		return c.Kind + ":" + hex.EncodeToString(sum[:])
	}
	return c.Kind + ":" + c.Text
}

// Read captures the current OS clipboard (text, image or file). The returned
// token is the pasteboard change count, which callers may use to skip a read
// when nothing has changed; it is 0 on platforms without a native reader.
func Read() (Capture, int64) { return readOSClipboard() }

var (
	linkRe = regexp.MustCompile(`^(https?|ftp|file)://\S+$`)
	wwwRe  = regexp.MustCompile(`^www\.\S+\.\S+$`)
	mailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

	// pngSignature is the 8-byte magic that starts every PNG stream.
	pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

	// codeHints are substrings that strongly suggest a code or structured
	// snippet rather than prose.
	codeHints = []string{
		"func ", "def ", "class ", "import ", "package ", "=>", "</", "};",
		"select ", "insert ", "update ", "delete ", "#!/", "{\n", "()", "::",
	}
)

// Normalize canonicalises a capture before it is compared or stored: CRLF and
// stray carriage returns become LF and trailing whitespace is trimmed so that
// trivial re-copies collapse into a single entry.
func Normalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimRight(text, " \t\n")
}

// DetectKind classifies a capture into one of the Kind* labels. The check is
// heuristic and deliberately conservative: anything ambiguous is plain text.
func DetectKind(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return KindText
	}
	if !strings.Contains(t, "\n") {
		if linkRe.MatchString(t) || wwwRe.MatchString(t) || mailRe.MatchString(t) {
			return KindLink
		}
	}
	if looksLikePaths(t) {
		return KindFile
	}
	if strings.Contains(t, "\n") && !strings.Contains(t, " ") {
		// Many short, space-free lines -> key material or a list, treat as code.
		if len(strings.Split(t, "\n")) >= 3 {
			return KindCode
		}
	}
	lower := strings.ToLower(t)
	for _, h := range codeHints {
		if strings.Contains(lower, h) {
			return KindCode
		}
	}
	if strings.ContainsAny(t, "{}();=") && strings.Contains(t, "\n") {
		return KindCode
	}
	return KindText
}

// looksLikePaths reports whether every non-empty line is a filesystem path,
// which is how a Finder copy of one or more files presents itself.
func looksLikePaths(text string) bool {
	lines := strings.Split(text, "\n")
	seen := 0
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		seen++
		if strings.ContainsAny(ln, " \t") {
			return false
		}
		if !(strings.HasPrefix(ln, "/") || strings.HasPrefix(ln, "~/") ||
			strings.HasPrefix(ln, "./") || strings.HasPrefix(ln, "../")) {
			return false
		}
	}
	return seen > 0
}

// ImageCapture builds an image capture from raw PNG bytes, deriving a text
// representation (dimensions, size, content hash) used for preview and
// de-duplication.
func ImageCapture(png []byte) Capture {
	sum := sha256.Sum256(png)
	w, h := pngSize(png)
	label := fmt.Sprintf("Image %dx%d (%s)", w, h, humanBytes(len(png)))
	if w <= 0 || h <= 0 {
		label = fmt.Sprintf("Image (%s)", humanBytes(len(png)))
	}
	return Capture{
		Kind: KindImage,
		Text: fmt.Sprintf("%s\nsha256:%s", label, hex.EncodeToString(sum[:])),
		Data: png,
		Mime: "image/png",
	}
}

// FileCapture builds a file capture from the given paths. Text holds the paths
// joined by newlines so the copy/export actions remain useful; an empty list
// yields a zero capture.
func FileCapture(paths []string) Capture {
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return Capture{}
	}
	return Capture{
		Kind: KindFile,
		Text: strings.Join(clean, "\n"),
	}
}

// TextCapture classifies an arbitrary text selection into a capture.
func TextCapture(text string) Capture {
	text = Normalize(text)
	if strings.TrimSpace(text) == "" {
		return Capture{}
	}
	return Capture{Kind: DetectKind(text), Text: text}
}

// PreviewFor chooses a list-view preview for a capture of the given kind/text.
func PreviewFor(kind, text string) string {
	switch kind {
	case KindImage:
		line, _, _ := strings.Cut(text, "\n")
		return line
	case KindFile:
		lines := strings.Split(text, "\n")
		base := baseName(lines[0])
		if len(lines) > 1 {
			return fmt.Sprintf("%s (+%d)", base, len(lines)-1)
		}
		return base
	}
	return Preview(text)
}

// baseName returns the final path element, tolerating both separators.
func baseName(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// humanBytes renders n bytes in a compact human-readable form.
func humanBytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Preview flattens a capture to a single line and truncates it for list views.
func Preview(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if flat == "" {
		return ""
	}
	r := []rune(flat)
	if len(r) > PreviewRunes {
		return string(r[:PreviewRunes]) + "…"
	}
	return flat
}

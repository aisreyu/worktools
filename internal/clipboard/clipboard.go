// Package clipboard holds the pure-text helpers shared by the clipboard
// manager: normalising captures, classifying their kind and building previews.
// Storage and encryption live in the vault; this package never touches disk.
package clipboard

import (
	"regexp"
	"strings"
)

// Kinds of capture the manager recognises.
const (
	KindText = "text" // free-form text
	KindLink = "link" // a URL or email address
	KindCode = "code" // source code / structured snippets
)

// PreviewRunes caps how many characters a preview keeps before eliding.
const PreviewRunes = 80

var (
	linkRe = regexp.MustCompile(`^(https?|ftp|file)://\S+$`)
	wwwRe  = regexp.MustCompile(`^www\.\S+\.\S+$`)
	mailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
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

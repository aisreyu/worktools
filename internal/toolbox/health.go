package toolbox

import (
	"sort"
	"strings"
	"time"
)

// PasswordAuditItem is one credential to audit. Password must be decrypted by
// the caller (the vault service) before being passed in.
type PasswordAuditItem struct {
	ID        int64  `json:"id"`
	Source    string `json:"source"` // "url" | "secret"
	Title     string `json:"title"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	UpdatedAt int64  `json:"updatedAt"` // unix seconds
}

// PasswordAuditFinding describes one problem (or note) for a credential.
type PasswordAuditFinding struct {
	Source   string `json:"source"`
	Title    string `json:"title"`
	Username string `json:"username"`
	Issue    string `json:"issue"`    // weak | reused | old
	Severity string `json:"severity"` // high | medium | low
	Detail   string `json:"detail"`
}

// PasswordAuditReport summarises a password-health audit.
type PasswordAuditReport struct {
	Total    int                    `json:"total"`
	Weak     int                    `json:"weak"`
	Reused   int                    `json:"reused"`
	Old      int                    `json:"old"`
	Findings []PasswordAuditFinding `json:"findings"`
}

// staleAfter is how long a password may go unchanged before it is flagged.
const staleAfter = 90 * 24 * time.Hour

// commonPasswords is a small deny-list of the most abused passwords.
var commonPasswords = map[string]bool{
	"123456": true, "123456789": true, "password": true, "qwerty": true,
	"12345678": true, "111111": true, "123123": true, "abc123": true,
	"admin": true, "admin123": true, "root": true, "letmein": true,
	"welcome": true, "iloveyou": true, "1q2w3e4r": true, "qwerty123": true,
	"000000": true, "888888": true, "666666": true, "passw0rd": true,
}

// AuditPasswords checks a set of credentials for weak, reused and stale
// passwords. now is injected for deterministic testing.
func AuditPasswords(items []PasswordAuditItem, now time.Time) PasswordAuditReport {
	rep := PasswordAuditReport{Total: len(items), Findings: make([]PasswordAuditFinding, 0)}

	// Count password reuse across the whole set.
	uses := map[string]int{}
	for _, it := range items {
		if it.Password == "" {
			continue
		}
		uses[it.Password]++
	}

	for _, it := range items {
		if strings.TrimSpace(it.Password) == "" {
			rep.Findings = append(rep.Findings, PasswordAuditFinding{
				Source: it.Source, Title: it.Title, Username: it.Username,
				Issue: "weak", Severity: "high", Detail: "empty_password",
			})
			rep.Weak++
			continue
		}
		if why, weak := weakReason(it); weak {
			rep.Weak++
			rep.Findings = append(rep.Findings, PasswordAuditFinding{
				Source: it.Source, Title: it.Title, Username: it.Username,
				Issue: "weak", Severity: "high", Detail: why,
			})
		}
		if uses[it.Password] > 1 {
			rep.Reused++
			rep.Findings = append(rep.Findings, PasswordAuditFinding{
				Source: it.Source, Title: it.Title, Username: it.Username,
				Issue: "reused", Severity: "medium",
				Detail: "reused:" + itoa(uses[it.Password]),
			})
		}
		if it.UpdatedAt > 0 && now.Sub(time.Unix(it.UpdatedAt, 0)) > staleAfter {
			rep.Old++
			days := int(now.Sub(time.Unix(it.UpdatedAt, 0)).Hours() / 24)
			rep.Findings = append(rep.Findings, PasswordAuditFinding{
				Source: it.Source, Title: it.Title, Username: it.Username,
				Issue: "old", Severity: "low",
				Detail: "stale:" + itoa(days),
			})
		}
	}
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		return severityRank(rep.Findings[i].Severity) > severityRank(rep.Findings[j].Severity)
	})
	return rep
}

// weakReason returns the first weakness found, or false when the password is
// reasonably strong.
func weakReason(it PasswordAuditItem) (string, bool) {
	p := it.Password
	if len(p) < 8 {
		return "too_short", true
	}
	if commonPasswords[strings.ToLower(p)] {
		return "common_password", true
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range p {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	classes := 0
	for _, b := range []bool{hasUpper, hasLower, hasDigit, hasSymbol} {
		if b {
			classes++
		}
	}
	if classes < 2 {
		return "few_classes", true
	}
	if it.Title != "" && strings.Contains(strings.ToLower(p), strings.ToLower(it.Title)) {
		return "contains_title", true
	}
	if it.Username != "" && strings.EqualFold(p, it.Username) {
		return "same_as_username", true
	}
	return "", false
}

func severityRank(s string) int {
	switch s {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

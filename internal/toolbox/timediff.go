package toolbox

import (
	"fmt"
	"strings"
	"time"
)

// TimeDiffResult describes the interval between two timestamps in several units.
type TimeDiffResult struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Seconds   int64  `json:"seconds"`
	Humanized string `json:"humanized"`
	AbsHuman  string `json:"absHumanized"`
}

var timeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02",
}

// parseTimeFlexible tries several common layouts plus unix seconds/millis.
func parseTimeFlexible(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	if n, ok := parseAllInt(s); ok {
		switch len(strings.TrimPrefix(s, "-")) {
		case 13:
			return time.UnixMilli(n), nil
		case 10:
			return time.Unix(n, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time %q", s)
}

func parseAllInt(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	var n int64
	neg := false
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int64(s[i]-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

// TimeDiff computes the signed interval between two timestamps.
func TimeDiff(from, to string) (TimeDiffResult, error) {
	a, err := parseTimeFlexible(from)
	if err != nil {
		return TimeDiffResult{}, fmt.Errorf("from: %w", err)
	}
	b, err := parseTimeFlexible(to)
	if err != nil {
		return TimeDiffResult{}, fmt.Errorf("to: %w", err)
	}
	d := b.Sub(a)
	return TimeDiffResult{
		From:      a.Format(time.RFC3339),
		To:        b.Format(time.RFC3339),
		Seconds:   int64(d.Seconds()),
		Humanized: humanizeDuration(d),
		AbsHuman:  humanizeDuration(absDuration(d)),
	}, nil
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// humanizeDuration renders a duration as "3d 4h 5m 6s" style components.
func humanizeDuration(d time.Duration) string {
	neg := ""
	if d < 0 {
		neg = "-"
		d = -d
	}
	d = d.Round(time.Second)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	d -= mins * time.Minute
	secs := d / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 || len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	parts = append(parts, fmt.Sprintf("%ds", secs))
	return neg + strings.Join(parts, " ")
}

// Now returns the current local time formatted as RFC3339 (used by the UI for
// "now" quick-fill).
func Now() string { return time.Now().Format(time.RFC3339) }

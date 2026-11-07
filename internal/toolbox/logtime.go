package toolbox

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LogTimeFormat describes a recognised timestamp format.
type LogTimeFormat struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Layout   string `json:"layout"`
	Regex    string `json:"regex"`
	HasTZ    bool   `json:"hasTz"`
}

// LogTimeMatch reports where a timestamp was found inside a line.
type LogTimeMatch struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Text    string `json:"text"`
	Unix    int64  `json:"unix"`      // seconds
	UnixMS  int64  `json:"unixMs"`    // milliseconds
	RFC3339 string `json:"rfc3339"`   // local-time RFC3339
	FormatID string `json:"formatId"`
}

// LogTimeResult is the outcome of detecting/converting timestamps in text.
type LogTimeResult struct {
	FormatID  string        `json:"formatId"`
	Format    string        `json:"format"`
	Matches   []LogTimeMatch `json:"matches"`
	Converted string        `json:"converted"`
	Count     int           `json:"count"`
}

// logFormats is ordered from most to least specific; the first match wins.
var logFormats = []LogTimeFormat{
	{ID: "rfc3339", Label: "RFC3339 (2006-01-02T15:04:05Z07:00)", Layout: time.RFC3339, Regex: `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})`, HasTZ: true},
	{ID: "datetime_ms", Label: "2006-01-02 15:04:05.000", Layout: "2006-01-02 15:04:05.000", Regex: `\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}\.\d{1,9}`},
	{ID: "datetime", Label: "2006-01-02 15:04:05", Layout: "2006-01-02 15:04:05", Regex: `\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}`},
	{ID: "date_only", Label: "2006-01-02", Layout: "2006-01-02", Regex: `\d{4}-\d{2}-\d{2}`},
	{ID: "slash_datetime", Label: "2006/01/02 15:04:05", Layout: "2006/01/02 15:04:05", Regex: `\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`},
	{ID: "apache", Label: "02/Jan/2006:15:04:05 -0700", Layout: "02/Jan/2006:15:04:05 -0700", Regex: `\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}`, HasTZ: true},
	{ID: "syslog", Label: "Jan _2 15:04:05", Layout: "Jan _2 15:04:05", Regex: `[A-Z][a-z]{2} +\d{1,2} \d{2}:\d{2}:\d{2}`},
	{ID: "iso_compact", Label: "20060102T150405", Layout: "20060102T150405", Regex: `\d{8}T\d{6}`},
}

var (
	reEpochMS = regexp.MustCompile(`\b(1[0-9]{12}|[2-9][0-9]{11})\b`)
	reEpochS  = regexp.MustCompile(`\b(1[0-9]{9}|[2-9][0-9]{8})\b`)
)

// DetectLogTime finds the most likely timestamp format in text, or "epoch" if
// only numeric epochs are present.
func DetectLogTime(text string) LogTimeFormat {
	for _, f := range logFormats {
		if regexp.MustCompile(f.Regex).MatchString(text) {
			return f
		}
	}
	if reEpochMS.MatchString(text) {
		return LogTimeFormat{ID: "epoch_ms", Label: "epoch_ms", Layout: "unix_ms", Regex: reEpochMS.String()}
	}
	if reEpochS.MatchString(text) {
		return LogTimeFormat{ID: "epoch_s", Label: "epoch_s", Layout: "unix_s", Regex: reEpochS.String()}
	}
	return LogTimeFormat{ID: "", Label: "unknown", Layout: ""}
}

// ConvertLogTime detects the timestamp format in text (or uses formatID when
// provided) and rewrites every timestamp according to output: "epoch" (unix
// seconds), "epoch_ms", "rfc3339", or "local" (2006-01-02 15:04:05).
func ConvertLogTime(text string, formatID, output string) (LogTimeResult, error) {
	defaultOffset := time.Now().Unix()
	loc := time.Local

	var re *regexp.Regexp
	var format LogTimeFormat
	switch {
	case formatID == "epoch_s" || formatID == "epoch_ms":
		format = LogTimeFormat{ID: formatID, Label: formatID}
		if formatID == "epoch_ms" {
			re = reEpochMS
		} else {
			re = reEpochS
		}
	default:
		if formatID != "" {
			for _, f := range logFormats {
				if f.ID == formatID {
					format = f
					break
				}
			}
		}
		if format.ID == "" {
			format = DetectLogTime(text)
		}
		if format.ID == "" {
			return LogTimeResult{}, fmt.Errorf("logtime_no_match")
		}
		if format.ID == "epoch_ms" {
			re = reEpochMS
		} else if format.ID == "epoch_s" {
			re = reEpochS
		} else {
			re = regexp.MustCompile(format.Regex)
		}
	}
	if re == nil {
		return LogTimeResult{}, fmt.Errorf("logtime_no_match")
	}

	res := LogTimeResult{FormatID: format.ID, Format: format.Label}
	locs := re.FindAllStringIndex(text, -1)
	var b strings.Builder
	last := 0
	delta := 0
	for _, l := range locs {
		frag := text[l[0]:l[1]]
		t, ok := parseLogTime(frag, format, defaultOffset)
		if !ok {
			continue
		}
		replacement := formatLogOutput(t, output, loc)
		b.WriteString(text[last:l[0]])
		b.WriteString(replacement)
		last = l[1]

		res.Matches = append(res.Matches, LogTimeMatch{
			Start:    l[0] + delta,
			End:      l[0] + delta + len(replacement),
			Text:     frag,
			Unix:     t.Unix(),
			UnixMS:   t.UnixMilli(),
			RFC3339:  t.In(loc).Format(time.RFC3339),
			FormatID: format.ID,
		})
		delta += len(replacement) - len(frag)
	}
	b.WriteString(text[last:])
	res.Converted = b.String()
	res.Count = len(res.Matches)
	return res, nil
}

func parseLogTime(frag string, format LogTimeFormat, defaultOffset int64) (time.Time, bool) {
	switch format.ID {
	case "epoch_s":
		n, err := strconv.ParseInt(strings.TrimSpace(frag), 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(n, 0), true
	case "epoch_ms":
		n, err := strconv.ParseInt(strings.TrimSpace(frag), 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.UnixMilli(n), true
	}
	// Normalise a space separator to 'T' for RFC3339 only layouts.
	for _, layout := range layoutCandidates(format) {
		if t, err := time.Parse(layout, frag); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func layoutCandidates(format LogTimeFormat) []string {
	layouts := []string{format.Layout}
	if strings.Contains(format.Layout, "T") {
		layouts = append(layouts, strings.Replace(format.Layout, "T", " ", 1))
	}
	if strings.Contains(format.Layout, " ") && format.HasTZ {
		layouts = append(layouts, strings.Replace(format.Layout, " ", "T", 1))
	}
	return layouts
}

func formatLogOutput(t time.Time, output string, loc *time.Location) string {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "epoch":
		return strconv.FormatInt(t.Unix(), 10)
	case "epoch_ms":
		return strconv.FormatInt(t.UnixMilli(), 10)
	case "rfc3339":
		return t.In(loc).Format(time.RFC3339)
	default: // local
		return t.In(loc).Format("2006-01-02 15:04:05")
	}
}

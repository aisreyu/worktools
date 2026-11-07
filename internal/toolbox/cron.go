package toolbox

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSchedule is a parsed 5-field cron expression.
type CronSchedule struct {
	Expression string `json:"expression"`
	Minute     string `json:"minute"`
	Hour       string `json:"hour"`
	DayOfMonth string `json:"dayOfMonth"`
	Month      string `json:"month"`
	DayOfWeek  string `json:"dayOfWeek"`
	// Description is a machine-readable summary code the frontend maps to
	// localized human-readable text, e.g. "every_day:09:00".
	Description string `json:"description"`
	// NextRuns holds the next few firing times for preview.
	NextRuns []string `json:"nextRuns"`
}

type cronField struct {
	allowed map[int]bool
	star    bool
}

var cronMacros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

var cronMonthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var cronDayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// ParseCron parses a standard 5-field cron expression (minute hour day-of-month
// month day-of-week) plus the common @macros, and previews upcoming runs.
func ParseCron(expr string, count int) (CronSchedule, error) {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return CronSchedule{}, fmt.Errorf("cron_empty")
	}
	normalized := raw
	if m, ok := cronMacros[strings.ToLower(raw)]; ok {
		normalized = m
	}
	fields := strings.Fields(normalized)
	if len(fields) != 5 {
		return CronSchedule{}, fmt.Errorf("cron_fields: %d", len(fields))
	}

	minute, err := parseCronField(fields[0], 0, 59, nil)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("cron_minute: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23, nil)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("cron_hour: %w", err)
	}
	dom, err := parseCronField(fields[2], 1, 31, nil)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("cron_dom: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12, cronMonthNames)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("cron_month: %w", err)
	}
	dow, err := parseCronField(fields[4], 0, 6, cronDayNames)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("cron_dow: %w", err)
	}

	if count <= 0 || count > 20 {
		count = 5
	}
	runs := nextCronRuns(minute, hour, dom, month, dow, time.Now(), count)
	pretty := make([]string, 0, len(runs))
	for _, t := range runs {
		pretty = append(pretty, t.Format("2006-01-02 15:04 Mon"))
	}

	return CronSchedule{
		Expression:  raw,
		Minute:      fields[0],
		Hour:        fields[1],
		DayOfMonth:  fields[2],
		Month:       fields[3],
		DayOfWeek:   fields[4],
		Description: cronDescriptionCode(minute, hour, dom, month, dow),
		NextRuns:    pretty,
	}, nil
}

// cronDescriptionCode builds a compact summary code for the frontend to
// localize. Recognized shapes: every_minute, hourly, every_day, every_weekday,
// weekly, monthly, yearly, complex.
func cronDescriptionCode(minute, hour, dom, month, dow cronField) string {
	if minute.star && hour.star && dom.star && month.star && dow.star {
		return "every_minute"
	}
	singleTime := len(minute.allowed) == 1 && len(hour.allowed) == 1
	if hour.star && dom.star && month.star && dow.star && len(minute.allowed) == 1 {
		return "hourly:" + cronFirstValue(minute)
	}
	if dom.star && month.star && singleTime {
		if dow.star {
			return "every_day:" + cronFirstValue(hour) + ":" + cronFirstValue(minute)
		}
		if isWeekdayRange(dow) {
			return "every_weekday:" + cronFirstValue(hour) + ":" + cronFirstValue(minute)
		}
		if len(dow.allowed) == 1 {
			return "weekly:" + cronFirstValue(dow) + ":" + cronFirstValue(hour) + ":" + cronFirstValue(minute)
		}
	}
	if month.star && dow.star && len(dom.allowed) == 1 && singleTime {
		return "monthly:" + cronFirstValue(dom) + ":" + cronFirstValue(hour) + ":" + cronFirstValue(minute)
	}
	if dow.star && len(month.allowed) == 1 && len(dom.allowed) == 1 && singleTime {
		return "yearly:" + cronFirstValue(month) + ":" + cronFirstValue(dom) + ":" + cronFirstValue(hour) + ":" + cronFirstValue(minute)
	}
	return "complex"
}

func cronFirstValue(f cronField) string {
	min := -1
	for v, ok := range f.allowed {
		if ok && (min == -1 || v < min) {
			min = v
		}
	}
	if min == -1 {
		return "*"
	}
	return strconv.Itoa(min)
}

// isWeekdayRange reports whether the day-of-week field covers exactly mon-fri.
func isWeekdayRange(dow cronField) bool {
	if len(dow.allowed) != 5 {
		return false
	}
	for d := 1; d <= 5; d++ {
		if !dow.allowed[d] {
			return false
		}
	}
	return true
}

func parseCronField(spec string, min, max int, names map[string]int) (cronField, error) {
	f := cronField{allowed: map[int]bool{}, star: spec == "*" || spec == "?"}
	norm := strings.ToLower(strings.TrimSpace(spec))
	if norm == "*" || norm == "?" {
		for i := min; i <= max; i++ {
			f.allowed[i] = true
		}
		return f, nil
	}

	for _, part := range strings.Split(norm, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		step := 1
		if i := strings.Index(part, "/"); i >= 0 {
			s := strings.TrimSpace(part[i+1:])
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				return f, fmt.Errorf("cron_bad_step: %s", part)
			}
			step = n
			part = strings.TrimSpace(part[:i])
		}

		lo, hi := min, max
		switch {
		case part == "*" || part == "":
			// full range
		case strings.Contains(part, "-"):
			bounds := strings.SplitN(part, "-", 2)
			var err error
			lo, err = cronValue(bounds[0], names)
			if err != nil {
				return f, err
			}
			hi, err = cronValue(bounds[1], names)
			if err != nil {
				return f, err
			}
			if lo > hi {
				return f, fmt.Errorf("cron_bad_range: %s", part)
			}
		default:
			v, err := cronValue(part, names)
			if err != nil {
				return f, err
			}
			lo, hi = v, v
		}
		if lo < min || hi > max {
			return f, fmt.Errorf("cron_out_of_range: %s", part)
		}
		for v := lo; v <= hi; v += step {
			f.allowed[v] = true
		}
	}
	if len(f.allowed) == 0 {
		return f, fmt.Errorf("cron_empty_field: %s", spec)
	}
	return f, nil
}

func cronValue(s string, names map[string]int) (int, error) {
	s = strings.TrimSpace(s)
	if names != nil {
		if v, ok := names[s]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("cron_bad_value: %s", s)
	}
	// cron tolerates 7 as Sunday
	if names != nil && v == 7 {
		return 0, nil
	}
	return v, nil
}

func nextCronRuns(minute, hour, dom, month, dow cronField, from time.Time, count int) []time.Time {
	var out []time.Time
	// Start at the next whole minute.
	t := from.Truncate(time.Minute).Add(time.Minute)
	limit := from.AddDate(5, 0, 0)
	for len(out) < count && t.Before(limit) {
		if cronMatches(minute, hour, dom, month, dow, t) {
			out = append(out, t)
		}
		t = t.Add(time.Minute)
	}
	return out
}

func cronMatches(minute, hour, dom, month, dow cronField, t time.Time) bool {
	if !minute.allowed[t.Minute()] || !hour.allowed[t.Hour()] || !month.allowed[int(t.Month())] {
		return false
	}
	domHit := dom.allowed[t.Day()]
	dowHit := dow.allowed[int(t.Weekday())]
	// Standard cron: when both day fields are restricted, either may match.
	if dom.star && dow.star {
		return true
	}
	if dom.star {
		return dowHit
	}
	if dow.star {
		return domHit
	}
	return domHit || dowHit
}

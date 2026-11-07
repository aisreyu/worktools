// Package holiday provides China statutory holiday and adjusted-workday data
// for calendar display. Data covers 2024-2027; 2027 is predicted (the State
// Council usually publishes the next year's schedule in November).
package holiday

import "time"

// Kind describes a special day.
type Kind int

const (
	// KindNone is a regular day.
	KindNone Kind = iota
	// KindHoliday is a statutory holiday (day off).
	KindHoliday
	// KindWorkday is a weekend converted to a workday (调休上班).
	KindWorkday
)

// Day is one special day.
type Day struct {
	Date   time.Time `json:"date"`   // local midnight
	Kind   Kind      `json:"kind"`   // holiday | workday
	Name   string    `json:"name"`   // festival name, e.g. "元旦"
	NameEn string    `json:"nameEn"` // English name
}

// String implements fmt.Stringer.
func (d Day) String() string {
	if d.Kind == KindWorkday {
		return d.NameEn
	}
	return d.Name
}

// special maps "2006-01-02" to the day info.
var special = map[string]Day{}

func add(y int, m time.Month, d int, kind Kind, name, nameEn string) {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	special[t.Format("2006-01-02")] = Day{Date: t, Kind: kind, Name: name, NameEn: nameEn}
}

// holidayRange adds each day in [start, end] as a holiday.
func holidayRange(y int, sm, sd, em, ed int, name, nameEn string) {
	start := time.Date(y, time.Month(sm), sd, 0, 0, 0, 0, time.Local)
	end := time.Date(y, time.Month(em), ed, 0, 0, 0, 0, time.Local)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		add(d.Year(), d.Month(), d.Day(), KindHoliday, name, nameEn)
	}
}

func init() {
	// ---- 2024 (国务院办公厅 2023-10-25 发布) ----
	holidayRange(2024, 1, 1, 1, 1, "元旦", "New Year's Day")
	add(2024, 2, 4, KindWorkday, "调休上班", "Adjusted workday")
	add(2024, 2, 18, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2024, 2, 10, 2, 17, "春节", "Spring Festival")
	add(2024, 4, 7, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2024, 4, 4, 4, 6, "清明节", "Qingming Festival")
	add(2024, 4, 28, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2024, 5, 1, 5, 5, "劳动节", "Labor Day")
	add(2024, 9, 14, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2024, 9, 15, 9, 17, "中秋节", "Mid-Autumn Festival")
	add(2024, 9, 29, KindWorkday, "调休上班", "Adjusted workday")
	add(2024, 10, 12, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2024, 10, 1, 10, 7, "国庆节", "National Day")

	// ---- 2025 (2024-11-12 发布) ----
	add(2025, 1, 26, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2025, 1, 28, 2, 4, "春节", "Spring Festival")
	add(2025, 2, 8, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2025, 4, 4, 4, 6, "清明节", "Qingming Festival")
	holidayRange(2025, 5, 1, 5, 5, "劳动节", "Labor Day")
	add(2025, 9, 28, KindWorkday, "调休上班", "Adjusted workday")
	add(2025, 10, 11, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2025, 10, 1, 10, 8, "国庆节+中秋节", "National Day & Mid-Autumn")

	// ---- 2026 (2025-11 发布) ----
	holidayRange(2026, 1, 1, 1, 3, "元旦", "New Year's Day")
	add(2026, 2, 15, KindWorkday, "调休上班", "Adjusted workday")
	add(2026, 2, 28, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2026, 2, 16, 2, 24, "春节", "Spring Festival")
	holidayRange(2026, 4, 4, 4, 6, "清明节", "Qingming Festival")
	add(2026, 4, 26, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2026, 5, 1, 5, 5, "劳动节", "Labor Day")
	add(2026, 6, 19, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2026, 6, 19, 6, 21, "端午节", "Dragon Boat Festival")
	add(2026, 9, 27, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2026, 9, 25, 9, 27, "中秋节", "Mid-Autumn Festival")
	add(2026, 10, 10, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2026, 10, 1, 10, 8, "国庆节", "National Day")

	// ---- 2027 (预测，官方通常 11 月发布) ----
	holidayRange(2027, 1, 1, 1, 3, "元旦", "New Year's Day")
	add(2027, 2, 20, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2027, 2, 6, 2, 12, "春节", "Spring Festival")
	holidayRange(2027, 4, 4, 4, 6, "清明节", "Qingming Festival")
	holidayRange(2027, 5, 1, 5, 5, "劳动节", "Labor Day")
	holidayRange(2027, 6, 9, 6, 11, "端午节", "Dragon Boat Festival")
	holidayRange(2027, 9, 15, 9, 17, "中秋节", "Mid-Autumn Festival")
	add(2027, 9, 18, KindWorkday, "调休上班", "Adjusted workday")
	holidayRange(2027, 10, 1, 10, 7, "国庆节", "National Day")
	add(2027, 10, 9, KindWorkday, "调休上班", "Adjusted workday")
}

// Lookup returns the special-day info for a date (kind None if ordinary).
func Lookup(t time.Time) Day {
	key := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
	if d, ok := special[key]; ok {
		return d
	}
	return Day{Date: t, Kind: KindNone}
}

// IsWorkday reports whether a date is a working day (weekday and not a
// statutory holiday, or an adjusted workday weekend).
func IsWorkday(t time.Time) bool {
	d := Lookup(t)
	if d.Kind == KindWorkday {
		return true
	}
	if d.Kind == KindHoliday {
		return false
	}
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// Month returns all special days in the month of t, sorted by date.
func Month(t time.Time) []Day {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	out := []Day{}
	for d := first; d.Month() == t.Month(); d = d.AddDate(0, 0, 1) {
		if info := Lookup(d); info.Kind != KindNone {
			out = append(out, info)
		}
	}
	return out
}

// Years returns the covered years, sorted.
func Years() []int {
	seen := map[int]bool{}
	for _, d := range special {
		seen[d.Date.Year()] = true
	}
	out := []int{}
	for y := range seen {
		out = append(out, y)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

package holiday

import (
	"testing"
	"time"
)

func TestSpringFestival2024(t *testing.T) {
	// 2024-02-10 (Sat) starts the Spring Festival; 2024-02-04 (Sun) is a workday.
	d := Lookup(time.Date(2024, 2, 10, 12, 0, 0, 0, time.Local))
	if d.Kind != KindHoliday || d.Name != "春节" {
		t.Fatalf("2024-02-10 = %+v, want 春节 holiday", d)
	}
	w := Lookup(time.Date(2024, 2, 4, 12, 0, 0, 0, time.Local))
	if w.Kind != KindWorkday {
		t.Fatalf("2024-02-04 kind = %v, want workday", w.Kind)
	}
	if IsWorkday(time.Date(2024, 2, 4, 0, 0, 0, 0, time.Local)) != true {
		t.Fatal("2024-02-04 should be a workday")
	}
	if IsWorkday(time.Date(2024, 2, 10, 0, 0, 0, 0, time.Local)) != false {
		t.Fatal("2024-02-10 should not be a workday")
	}
}

func TestNationalDay2025(t *testing.T) {
	// 2025-10-01..08 holiday (merged with Mid-Autumn), 2025-09-28 and
	// 2025-10-11 are adjusted workdays.
	if Lookup(time.Date(2025, 10, 8, 0, 0, 0, 0, time.Local)).Kind != KindHoliday {
		t.Fatal("2025-10-08 should be a holiday")
	}
	if Lookup(time.Date(2025, 10, 9, 0, 0, 0, 0, time.Local)).Kind != KindNone {
		t.Fatal("2025-10-09 should be ordinary")
	}
	if Lookup(time.Date(2025, 9, 28, 0, 0, 0, 0, time.Local)).Kind != KindWorkday {
		t.Fatal("2025-09-28 should be an adjusted workday")
	}
}

func TestMonthListing(t *testing.T) {
	days := Month(time.Date(2026, 10, 15, 0, 0, 0, 0, time.Local))
	if len(days) != 9 { // 8 holiday days + 1 adjusted workday
		t.Fatalf("2026-10 special days = %d, want 9: %+v", len(days), days)
	}
	if days[0].Date.Day() != 1 || days[len(days)-1].Date.Day() != 10 {
		t.Fatalf("bounds wrong: first=%v last=%v", days[0].Date, days[len(days)-1].Date)
	}
}

func TestOrdinaryWeekend(t *testing.T) {
	// 2026-03-14 is a plain Saturday: not holiday, not workday.
	d := Lookup(time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local))
	if d.Kind != KindNone {
		t.Fatalf("2026-03-14 kind = %v, want none", d.Kind)
	}
	if IsWorkday(time.Date(2026, 3, 14, 0, 0, 0, 0, time.Local)) {
		t.Fatal("plain Saturday must not be a workday")
	}
}

func TestYearsSorted(t *testing.T) {
	ys := Years()
	for i := 1; i < len(ys); i++ {
		if ys[i] < ys[i-1] {
			t.Fatalf("Years not sorted: %v", ys)
		}
	}
	if len(ys) < 4 {
		t.Fatalf("expected at least 4 covered years, got %v", ys)
	}
}

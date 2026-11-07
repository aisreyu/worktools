package toolbox

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestConvertDataSize(t *testing.T) {
	cases := []struct {
		value    float64
		from, to string
		want     float64
	}{
		{1, "GiB", "MiB", 1024},
		{1, "GB", "MB", 1000},
		{1, "GiB", "GB", 1073741824.0 / 1e9},
		{1, "TiB", "GiB", 1024},
		{10, "GB", "GiB", 10e9 / float64(1<<30)},
	}
	for _, c := range cases {
		got, err := ConvertDataSize(c.value, c.from, c.to)
		if err != nil {
			t.Fatalf("ConvertDataSize(%v %s %s): %v", c.value, c.from, c.to, err)
		}
		if !approx(got.Value, c.want) {
			t.Errorf("ConvertDataSize(%v %s->%s) = %v, want %v", c.value, c.from, c.to, got.Value, c.want)
		}
	}
	// Human-readable summary should use the largest fitting unit.
	got, _ := ConvertDataSize(1, "GiB", "MiB")
	if got.Decimal == "" || got.Binary != "1 GiB" {
		t.Errorf("unexpected human summary: decimal=%q binary=%q", got.Decimal, got.Binary)
	}
}

func TestConvertDataSizeErrors(t *testing.T) {
	if _, err := ConvertDataSize(1, "nope", "MB"); err == nil {
		t.Error("expected error for unknown source unit")
	}
	if _, err := ConvertDataSize(1, "MB", "nope"); err == nil {
		t.Error("expected error for unknown target unit")
	}
}

func TestConvertDataRate(t *testing.T) {
	// 100 Mbps == 12.5 MB/s
	got, err := ConvertDataRate(100, "Mbps", "MB/s")
	if err != nil {
		t.Fatalf("ConvertDataRate: %v", err)
	}
	if !approx(got.Value, 12.5) {
		t.Errorf("100 Mbps -> MB/s = %v, want 12.5", got.Value)
	}
	// 1 Gbps == 1000 Mbps
	got2, _ := ConvertDataRate(1, "Gbps", "Mbps")
	if !approx(got2.Value, 1000) {
		t.Errorf("1 Gbps -> Mbps = %v, want 1000", got2.Value)
	}
}

func TestEstimateTransferTime(t *testing.T) {
	// 1 GB at 100 Mbps, 100% efficiency -> 80 s.
	got, err := EstimateTransferTime(1, "GB", 100, "Mbps", 100)
	if err != nil {
		t.Fatalf("EstimateTransferTime: %v", err)
	}
	if !approx(got.Seconds, 80) {
		t.Errorf("1 GB @ 100 Mbps = %v s, want 80", got.Seconds)
	}
	if got.Duration != "1m 20s" {
		t.Errorf("duration = %q, want %q", got.Duration, "1m 20s")
	}
	// 50% efficiency doubles the time.
	got2, _ := EstimateTransferTime(1, "GB", 100, "Mbps", 50)
	if !approx(got2.Seconds, 160) {
		t.Errorf("1 GB @ 100 Mbps @50%% = %v s, want 160", got2.Seconds)
	}
	if _, err := EstimateTransferTime(1, "GB", 0, "Mbps", 100); err == nil {
		t.Error("expected error for zero speed")
	}
}

func TestCalcRAIDCapacity(t *testing.T) {
	r5, err := CalcRAIDCapacity(4, 2, "raid5")
	if err != nil {
		t.Fatalf("raid5: %v", err)
	}
	if !approx(r5.RawTB, 8) || !approx(r5.UsableTB, 6) || r5.Tolerance != 1 {
		t.Errorf("raid5 4x2TB = raw %v usable %v tol %d, want 8/6/1", r5.RawTB, r5.UsableTB, r5.Tolerance)
	}
	r6, _ := CalcRAIDCapacity(6, 1, "6")
	if !approx(r6.UsableTB, 4) || r6.Tolerance != 2 {
		t.Errorf("raid6 6x1TB usable=%v tol=%d, want 4/2", r6.UsableTB, r6.Tolerance)
	}
	r10, _ := CalcRAIDCapacity(4, 4, "raid10")
	if !approx(r10.UsableTB, 8) {
		t.Errorf("raid10 4x4TB usable=%v, want 8", r10.UsableTB)
	}
	if _, err := CalcRAIDCapacity(2, 1, "raid5"); err == nil {
		t.Error("raid5 with 2 disks should fail")
	}
	if _, err := CalcRAIDCapacity(5, 1, "raid10"); err == nil {
		t.Error("raid10 with odd disks should fail")
	}
	if _, err := CalcRAIDCapacity(4, 1, "raid99"); err == nil {
		t.Error("unknown level should fail")
	}
}

func TestEstimateTraffic(t *testing.T) {
	// 100 Mbps at 100% for 1 day -> 100e6/8*86400 = 1.08e12 bytes.
	got, err := EstimateTraffic(100, 100, 1)
	if err != nil {
		t.Fatalf("EstimateTraffic: %v", err)
	}
	if !approx(got.PerDayBytes, 1.08e12) {
		t.Errorf("perDay = %v, want 1.08e12", got.PerDayBytes)
	}
	// 50% utilisation halves it.
	got2, _ := EstimateTraffic(100, 50, 2)
	if !approx(got2.EffectiveMbps, 50) {
		t.Errorf("effective = %v, want 50", got2.EffectiveMbps)
	}
	if !approx(got2.TotalBytes, 2*5.4e11) {
		t.Errorf("total over 2 days = %v, want 1.08e12", got2.TotalBytes)
	}
	if _, err := EstimateTraffic(0, 100, 1); err == nil {
		t.Error("zero bandwidth should fail")
	}
}

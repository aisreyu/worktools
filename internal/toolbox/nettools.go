package toolbox

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ---- data size units ----

type unitDef struct {
	canonical string  // display form, e.g. "GiB"
	factor    float64 // multiplier to the base unit (bytes for size, bits/s for rate)
}

// sizeDefs maps data-size units (lower-cased, no spaces) to their definition.
// Both decimal (SI, 1000-based) and binary (IEC, 1024-based) units are accepted.
var sizeDefs = map[string]unitDef{
	"b":   {"B", 1},
	"kb":  {"KB", 1e3},
	"mb":  {"MB", 1e6},
	"gb":  {"GB", 1e9},
	"tb":  {"TB", 1e12},
	"pb":  {"PB", 1e15},
	"eb":  {"EB", 1e18},
	"kib": {"KiB", 1 << 10},
	"mib": {"MiB", 1 << 20},
	"gib": {"GiB", 1 << 30},
	"tib": {"TiB", 1 << 40},
	"pib": {"PiB", 1 << 50},
	"eib": {"EiB", 1 << 60},
}

// rateDefs maps data-rate units to bits-per-second factors.
var rateDefs = map[string]unitDef{
	"bps":  {"bps", 1},
	"kbps": {"Kbps", 1e3},
	"mbps": {"Mbps", 1e6},
	"gbps": {"Gbps", 1e9},
	"tbps": {"Tbps", 1e12},
	"b/s":  {"B/s", 8},
	"kb/s": {"KB/s", 8e3},
	"mb/s": {"MB/s", 8e6},
	"gb/s": {"GB/s", 8e9},
	"tb/s": {"TB/s", 8e12},
}

func normalizeUnit(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}

func lookupUnit(defs map[string]unitDef, s string) (unitDef, error) {
	d, ok := defs[normalizeUnit(s)]
	if !ok {
		return unitDef{}, fmt.Errorf("unknown unit %q", s)
	}
	return d, nil
}

// trimNum renders a float without a trailing ".0" or scientific notation noise.
func trimNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// humanBytes renders a byte count using SI (1000) and IEC (1024) units.
func humanBytes(n float64) (decimal, binary string) {
	si := []string{"B", "KB", "MB", "GB", "PB", "EB"}
	ie := []string{"B", "KiB", "MiB", "GiB", "PiB", "EiB"}
	return humanize(n, 1000, si), humanize(n, 1024, ie)
}

// humanize picks the largest unit whose value is >= 1.
func humanize(n float64, base float64, units []string) string {
	f := math.Abs(n)
	i := 0
	for f >= base && i < len(units)-1 {
		f /= base
		n /= base
		i++
	}
	return round3(n) + " " + units[i]
}

// round3 keeps up to 3 significant decimals for display.
func round3(v float64) string {
	r := math.Round(v*1000) / 1000
	return trimNum(r)
}

// UnitConversion is the result of converting between two units of the same kind.
type UnitConversion struct {
	Value  float64 `json:"value"`  // converted value in the target unit
	Unit   string  `json:"unit"`   // canonical target unit
	Text   string  `json:"text"`   // e.g. "1.5 GB"
	Base   float64 `json:"base"`   // value normalised to the base unit (bytes or bits/s)
	Decimal string `json:"decimal"` // base rendered with SI units
	Binary  string `json:"binary"`  // base rendered with IEC units
}

// ConvertDataSize converts a data size between decimal and binary units.
func ConvertDataSize(value float64, from, to string) (UnitConversion, error) {
	fu, err := lookupUnit(sizeDefs, from)
	if err != nil {
		return UnitConversion{}, err
	}
	tu, err := lookupUnit(sizeDefs, to)
	if err != nil {
		return UnitConversion{}, err
	}
	bytes := value * fu.factor
	res := bytes / tu.factor
	dec, bin := humanBytes(bytes)
	return UnitConversion{
		Value:   res,
		Unit:    tu.canonical,
		Text:    round3(res) + " " + tu.canonical,
		Base:    bytes,
		Decimal: dec,
		Binary:  bin,
	}, nil
}

// ConvertDataRate converts a data rate between bit-based and byte-based units.
func ConvertDataRate(value float64, from, to string) (UnitConversion, error) {
	fu, err := lookupUnit(rateDefs, from)
	if err != nil {
		return UnitConversion{}, err
	}
	tu, err := lookupUnit(rateDefs, to)
	if err != nil {
		return UnitConversion{}, err
	}
	bps := value * fu.factor
	res := bps / tu.factor
	dec, bin := humanBytes(bps / 8) // bytes/sec for a friendly summary
	return UnitConversion{
		Value:   res,
		Unit:    tu.canonical,
		Text:    round3(res) + " " + tu.canonical,
		Base:    bps,
		Decimal: dec + "/s",
		Binary:  bin + "/s",
	}, nil
}

// TransferEstimate describes how long a payload takes over a link.
type TransferEstimate struct {
	Bytes       float64 `json:"bytes"`       // payload size in bytes
	Payload     string  `json:"payload"`     // e.g. "10 GB"
	BitsPerSec  float64 `json:"bitsPerSec"`  // effective link speed
	LinkSpeed   string  `json:"linkSpeed"`   // e.g. "100 Mbps"
	Seconds     float64 `json:"seconds"`     // raw seconds
	Duration    string  `json:"duration"`    // humanised, e.g. "13m 20s"
	SustainedBy string  `json:"sustainedBy"` // effective byte/s, e.g. "11.92 MB/s"
}

// EstimateTransferTime estimates the time to move size over a link, allowing for
// a usable-efficiency percentage (protocol overhead, contention, etc.).
func EstimateTransferTime(size float64, sizeUnit string, speed float64, speedUnit string, efficiencyPct float64) (TransferEstimate, error) {
	su, err := lookupUnit(sizeDefs, sizeUnit)
	if err != nil {
		return TransferEstimate{}, err
	}
	ru, err := lookupUnit(rateDefs, speedUnit)
	if err != nil {
		return TransferEstimate{}, err
	}
	if size < 0 || speed <= 0 {
		return TransferEstimate{}, fmt.Errorf("size and speed must be positive")
	}
	if efficiencyPct <= 0 || efficiencyPct > 100 {
		efficiencyPct = 100
	}
	bytes := size * su.factor
	bps := speed * ru.factor * (efficiencyPct / 100)
	secs := bytes * 8 / bps
	dec, _ := humanBytes(bps / 8)
	return TransferEstimate{
		Bytes:       bytes,
		Payload:     round3(size) + " " + su.canonical,
		BitsPerSec:  bps,
		LinkSpeed:   round3(speed) + " " + ru.canonical,
		Seconds:     secs,
		Duration:    humanizeDuration(time.Duration(secs * float64(time.Second))),
		SustainedBy: dec + "/s",
	}, nil
}

// RAIDPlan describes usable capacity for a RAID layout.
type RAIDPlan struct {
	Level     string  `json:"level"`
	Disks     int     `json:"disks"`
	DiskSize  float64 `json:"diskSize"`  // per-disk size in TB
	RawTB     float64 `json:"rawTB"`     // total raw capacity
	UsableTB  float64 `json:"usableTB"`  // capacity after redundancy
	LostTB    float64 `json:"lostTB"`    // capacity consumed by redundancy
	Efficiency float64 `json:"efficiency"` // usable / raw, 0-1
	Tolerance int     `json:"tolerance"`   // disk failures tolerated
}

// CalcRAIDCapacity computes usable capacity for common RAID levels.
func CalcRAIDCapacity(disks int, diskTB float64, level string) (RAIDPlan, error) {
	if disks <= 0 || diskTB <= 0 {
		return RAIDPlan{}, fmt.Errorf("disks and disk size must be positive")
	}
	lvl := strings.ToUpper(strings.TrimSpace(level))
	raw := float64(disks) * diskTB
	var usable float64
	var tol int
	switch lvl {
	case "0", "RAID0", "STRIPE":
		usable, tol = raw, 0
	case "1", "RAID1", "MIRROR":
		if disks < 2 {
			return RAIDPlan{}, fmt.Errorf("raid1 needs at least 2 disks")
		}
		usable, tol = raw/2, disks/2
	case "5", "RAID5":
		if disks < 3 {
			return RAIDPlan{}, fmt.Errorf("raid5 needs at least 3 disks")
		}
		usable, tol = raw*float64(disks-1)/float64(disks), 1
	case "6", "RAID6":
		if disks < 4 {
			return RAIDPlan{}, fmt.Errorf("raid6 needs at least 4 disks")
		}
		usable, tol = raw*float64(disks-2)/float64(disks), 2
	case "10", "RAID10", "1+0":
		if disks < 4 || disks%2 != 0 {
			return RAIDPlan{}, fmt.Errorf("raid10 needs an even number of disks (>= 4)")
		}
		usable, tol = raw/2, disks/2
	default:
		return RAIDPlan{}, fmt.Errorf("unsupported RAID level %q", level)
	}
	return RAIDPlan{
		Level:      lvl,
		Disks:      disks,
		DiskSize:   diskTB,
		RawTB:      raw,
		UsableTB:   usable,
		LostTB:     raw - usable,
		Efficiency: usable / raw,
		Tolerance:  tol,
	}, nil
}

// TrafficEstimate describes the data volume carried by a link over time.
type TrafficEstimate struct {
	BandwidthMbps float64 `json:"bandwidthMbps"`
	Utilization   float64 `json:"utilization"`   // percent
	EffectiveMbps float64 `json:"effectiveMbps"` // bandwidth * utilization
	Days          float64 `json:"days"`
	BytesPerSec   float64 `json:"bytesPerSec"`
	PerDayBytes   float64 `json:"perDayBytes"`
	TotalBytes    float64 `json:"totalBytes"`
	PerDayText    string  `json:"perDayText"`
	TotalText     string  `json:"totalText"`
	MonthlyBytes  float64 `json:"monthlyBytes"` // normalised to a 30-day month
	MonthlyText   string  `json:"monthlyText"`
}

// EstimateTraffic estimates the data volume carried by a link at a given
// utilisation over the supplied number of days.
func EstimateTraffic(bandwidthMbps, utilizationPct, days float64) (TrafficEstimate, error) {
	if bandwidthMbps <= 0 {
		return TrafficEstimate{}, fmt.Errorf("bandwidth must be positive")
	}
	if utilizationPct <= 0 || utilizationPct > 100 {
		utilizationPct = 100
	}
	if days <= 0 {
		days = 1
	}
	effMbps := bandwidthMbps * (utilizationPct / 100)
	bytesPerSec := effMbps * 1e6 / 8
	perDay := bytesPerSec * 86400
	total := perDay * days
	monthly := perDay * 30
	pd, _ := humanBytes(perDay)
	tt, _ := humanBytes(total)
	mt, _ := humanBytes(monthly)
	return TrafficEstimate{
		BandwidthMbps: bandwidthMbps,
		Utilization:   utilizationPct,
		EffectiveMbps: effMbps,
		Days:          days,
		BytesPerSec:   bytesPerSec,
		PerDayBytes:   perDay,
		TotalBytes:    total,
		PerDayText:    pd,
		TotalText:     tt,
		MonthlyBytes:  monthly,
		MonthlyText:   mt,
	}, nil
}

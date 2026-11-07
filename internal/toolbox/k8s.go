package toolbox

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Quantity is a parsed Kubernetes resource quantity.
type Quantity struct {
	Raw    string  `json:"raw"`
	Kind   string  `json:"kind"`   // cpu | memory | other
	Value  float64 `json:"value"`  // base unit: cores (cpu) or bytes (memory)
	Unit   string  `json:"unit"`   // normalised display unit
}

// K8sResourceEntry is one normalised cpu/memory quantity.
type K8sResourceEntry struct {
	Scope    string  `json:"scope"`    // requests | limits
	Resource string  `json:"resource"` // cpu | memory
	Raw      string  `json:"raw"`
	CPU       string `json:"cpu,omitempty"`      // e.g. "500m"
	CPUPlain  string `json:"cpuPlain,omitempty"` // e.g. "0.5"
	Memory    string `json:"memory,omitempty"`   // e.g. "512Mi"
	MemoryBytes int64 `json:"memoryBytes,omitempty"`
	Valid     bool   `json:"valid"`
	Note      string `json:"note,omitempty"`
}

// K8sValidationResult holds extracted resources and validation issues.
type K8sValidationResult struct {
	Entries []K8sResourceEntry `json:"entries"`
	Issues  []string           `json:"issues"`
	OK      bool               `json:"ok"`
}

var (
	cpuSuffixes    = map[string]float64{"n": 1e-9, "u": 1e-6, "m": 1e-3, "": 1}
	memBinSuffixes = map[string]float64{"Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40, "Pi": 1 << 50, "Ei": 1 << 60}
	memDecSuffixes = map[string]float64{"k": 1e3, "K": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15, "E": 1e18}
)

// ParseQuantity parses a Kubernetes quantity such as "500m", "2", "512Mi",
// "1.5Gi", "100M" or plain bytes.
func ParseQuantity(raw string) (Quantity, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Quantity{}, fmt.Errorf("k8s_empty_value")
	}
	// Split numeric prefix from suffix.
	idx := len(s)
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || (i == 0 && (r == '-' || r == '+')) {
			continue
		}
		idx = i
		break
	}
	numPart := s[:idx]
	suffix := strings.TrimSpace(s[idx:])
	num, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return Quantity{}, fmt.Errorf("k8s_bad_number: %s", raw)
	}
	// Memory (binary/decimal byte suffix) or plain bytes.
	if m, ok := memBinSuffixes[suffix]; ok {
		return Quantity{Raw: raw, Kind: "memory", Value: num * m, Unit: suffix}, nil
	}
	if m, ok := memDecSuffixes[suffix]; ok {
		return Quantity{Raw: raw, Kind: "memory", Value: num * m, Unit: suffix}, nil
	}
	if f, ok := cpuSuffixes[suffix]; ok {
		return Quantity{Raw: raw, Kind: "cpu", Value: num * f, Unit: suffix}, nil
	}
	return Quantity{}, fmt.Errorf("k8s_bad_unit: %s", suffix)
}

// FormatCPU renders cores (base unit) into the conventional Kubernetes form,
// using millicores when the value is not a whole number of cores.
func FormatCPU(cores float64) string {
	if cores == math.Trunc(cores) {
		return strconv.FormatInt(int64(cores), 10)
	}
	milli := cores * 1000
	if milli == math.Trunc(milli) {
		return strconv.FormatInt(int64(milli), 10) + "m"
	}
	return strconv.FormatFloat(milli, 'f', 3, 64) + "m"
}

// FormatMemoryBytes renders a byte count using the largest binary unit that
// divides it evenly, falling back to bytes.
func FormatMemoryBytes(bytes float64) string {
	units := []struct {
		suffix string
		size   float64
	}{{"Ei", 1 << 60}, {"Pi", 1 << 50}, {"Ti", 1 << 40}, {"Gi", 1 << 30}, {"Mi", 1 << 20}, {"Ki", 1 << 10}}
	for _, u := range units {
		if bytes >= u.size {
			v := bytes / u.size
			return trimFloat(v) + u.suffix
		}
	}
	return trimFloat(bytes)
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

// ConvertK8sResource converts a quantity to a target representation.
// For cpu, toUnit may be "cpu" (cores) or "m". For memory, toUnit may be one of
// Ki/Mi/Gi/Ti or "bytes".
func ConvertK8sResource(input, toUnit string) (string, error) {
	q, err := ParseQuantity(input)
	if err != nil {
		return "", err
	}
	switch q.Kind {
	case "cpu":
		switch toUnit {
		case "", "cpu", "cores":
			return strconv.FormatFloat(q.Value, 'f', 3, 64), nil
		case "m":
			return strconv.FormatFloat(q.Value*1000, 'f', 3, 64) + "m", nil
		default:
			return "", fmt.Errorf("k8s_bad_cpu_target")
		}
	case "memory":
		if toUnit == "" || toUnit == "bytes" || toUnit == "B" {
			return strconv.FormatInt(int64(math.Round(q.Value)), 10), nil
		}
		if m, ok := memBinSuffixes[toUnit]; ok {
			return trimFloat(q.Value/m) + toUnit, nil
		}
		if m, ok := memDecSuffixes[toUnit]; ok {
			return trimFloat(q.Value/m) + toUnit, nil
		}
		return "", fmt.Errorf("k8s_bad_mem_target: %s", toUnit)
	}
	return "", fmt.Errorf("k8s_unknown_kind")
}

// ValidateResourcesYAML scans a Kubernetes manifest for cpu/memory requests and
// limits, normalises them, and reports suspicious or invalid values.
func ValidateResourcesYAML(yaml string) K8sValidationResult {
	res := K8sValidationResult{OK: true}
	lines := strings.Split(yaml, "\n")
	scope := ""
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "requests":
			scope = "requests"
			continue
		case "limits":
			scope = "limits"
			continue
		}
		if scope == "" {
			continue
		}
		if key != "cpu" && key != "memory" {
			continue
		}
		val = strings.Trim(val, `"'`)
		entry := K8sResourceEntry{Scope: scope, Resource: key, Raw: val}
		q, err := ParseQuantity(val)
		if err != nil {
			res.OK = false
			entry.Note = err.Error()
			res.Issues = append(res.Issues, fmt.Sprintf("k8s_line_error:%d:%s.%s:%s:%s", i+1, scope, key, val, err.Error()))
			res.Entries = append(res.Entries, entry)
			continue
		}
		entry.Valid = true
		if key == "cpu" {
			entry.CPU = FormatCPU(q.Value)
			entry.CPUPlain = strconv.FormatFloat(q.Value, 'f', 3, 64)
		} else {
			entry.Memory = FormatMemoryBytes(q.Value)
			entry.MemoryBytes = int64(math.Round(q.Value))
			if q.Unit == "" {
				entry.Note = "k8s_mem_no_unit"
			}
		}
		res.Entries = append(res.Entries, entry)
	}
	// Cross-check: limits should not be below requests for the same resource.
	req := map[string]float64{}
	lim := map[string]float64{}
	for _, e := range res.Entries {
		q, err := ParseQuantity(e.Raw)
		if err != nil {
			continue
		}
		if e.Scope == "requests" {
			req[e.Resource] = q.Value
		} else {
			lim[e.Resource] = q.Value
		}
	}
	for r, lv := range lim {
		if rv, ok := req[r]; ok && lv < rv {
			res.OK = false
			res.Issues = append(res.Issues, fmt.Sprintf("k8s_limit_below_request:%s:%s:%s", r, FormatMemoryBytes(lv), FormatMemoryBytes(rv)))
		}
	}
	return res
}

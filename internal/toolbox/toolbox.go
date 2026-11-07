// Package toolbox provides stateless ops utilities exposed by the UI. None of
// these functions touch vault data.
package toolbox

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- encoding ----

// Base64Encode encodes s as standard base64.
func Base64Encode(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// Base64Decode decodes standard base64.
func Base64Decode(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	return string(b), err
}

// HexEncode encodes s as hex.
func HexEncode(s string) string { return hex.EncodeToString([]byte(s)) }

// HexDecode decodes hex.
func HexDecode(s string) (string, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	return string(b), err
}

// Hash returns the hex digest of s using the named algorithm.
func Hash(algo, s string) (string, error) {
	switch strings.ToLower(algo) {
	case "md5":
		h := md5.Sum([]byte(s))
		return hex.EncodeToString(h[:]), nil
	case "sha1":
		h := sha1.Sum([]byte(s))
		return hex.EncodeToString(h[:]), nil
	case "sha256":
		h := sha256.Sum256([]byte(s))
		return hex.EncodeToString(h[:]), nil
	default:
		return "", fmt.Errorf("unsupported hash %q", algo)
	}
}

// ---- json ----

// JSONFormat pretty-prints or compacts a JSON document.
func JSONFormat(input string, indent bool) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return "", err
	}
	if indent {
		b, err := json.MarshalIndent(v, "", "  ")
		return string(b), err
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// ---- time ----

// TimeFromUnix converts a unix timestamp (seconds or milliseconds) to RFC3339.
func TimeFromUnix(ts string, millis bool) (string, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64)
	if err != nil {
		return "", err
	}
	if millis {
		return time.UnixMilli(n).Format(time.RFC3339), nil
	}
	return time.Unix(n, 0).Format(time.RFC3339), nil
}

// UnixFromTime parses an RFC3339 timestamp and returns unix seconds.
func UnixFromTime(s string) (int64, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

// ---- net ----

// PortResult is the outcome of a single TCP port probe.
type PortResult struct {
	Port    int    `json:"port"`
	Open    bool   `json:"open"`
	Latency string `json:"latency"`
	Error   string `json:"error,omitempty"`
}

// PortScan probes the given TCP ports on host with a per-port timeout.
func PortScan(_ context.Context, host string, ports []int, timeoutMS int) []PortResult {
	if timeoutMS <= 0 {
		timeoutMS = 1500
	}
	results := make([]PortResult, 0, len(ports))
	for _, p := range ports {
		start := time.Now()
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutMS)*time.Millisecond)
		r := PortResult{Port: p, Latency: time.Since(start).Round(time.Millisecond).String()}
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Open = true
			_ = conn.Close()
		}
		results = append(results, r)
	}
	return results
}

// HTTPResult summarises an HTTP probe.
type HTTPResult struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Latency    string            `json:"latency"`
	Headers    map[string]string `json:"headers"`
}

// HTTPProbe issues a GET request and returns status, timing and headers.
func HTTPProbe(ctx context.Context, rawURL string, timeoutMS int) (HTTPResult, error) {
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	client := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return HTTPResult{}, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return HTTPResult{}, err
	}
	defer resp.Body.Close()

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[k] = strings.Join(v, ", ")
	}
	return HTTPResult{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Latency:    time.Since(start).Round(time.Millisecond).String(),
		Headers:    headers,
	}, nil
}

// DNS lookup types.
const (
	DNSA     = "A"
	DNSCNAME = "CNAME"
	DNSMX    = "MX"
	DNSTXT   = "TXT"
	DNSNS    = "NS"
)

// DNSLookup resolves name for the requested record type.
func DNSLookup(_ context.Context, name, record string) ([]string, error) {
	r := net.DefaultResolver
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch strings.ToUpper(record) {
	case DNSA, "AAAA", "":
		ips, err := r.LookupHost(ctx, name)
		if err != nil {
			return nil, err
		}
		sort.Strings(ips)
		return ips, nil
	case DNSCNAME:
		cname, err := r.LookupCNAME(ctx, name)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil
	case DNSMX:
		mxs, err := r.LookupMX(ctx, name)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(mxs))
		for _, m := range mxs {
			out = append(out, fmt.Sprintf("%s (pref %d)", m.Host, m.Pref))
		}
		return out, nil
	case DNSTXT:
		return r.LookupTXT(ctx, name)
	case DNSNS:
		nss, err := r.LookupNS(ctx, name)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(nss))
		for _, ns := range nss {
			out = append(out, ns.Host)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported record type %q", record)
	}
}

// SubnetInfo describes a parsed CIDR block.
type SubnetInfo struct {
	CIDR       string `json:"cidr"`
	Network    string `json:"network"`
	Mask       string `json:"mask"`
	Broadcast  string `json:"broadcast"`
	FirstHost  string `json:"firstHost"`
	LastHost   string `json:"lastHost"`
	HostCount  int64  `json:"hostCount"`
	Prefix     int    `json:"prefix"`
}

// ParseSubnet computes details of an IPv4 CIDR block.
func ParseSubnet(cidr string) (SubnetInfo, error) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return SubnetInfo{}, err
	}
	v4 := ip.To4()
	if v4 == nil {
		return SubnetInfo{}, fmt.Errorf("only IPv4 CIDR is supported")
	}
	network := ipnet.IP.To4()
	mask := net.IP(ipnet.Mask).String()
	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones

	broadcast := make(net.IP, len(network))
	first := make(net.IP, len(network))
	last := make(net.IP, len(network))
	copy(broadcast, network)
	copy(first, network)
	copy(last, network)
	for i := 0; i < len(network); i++ {
		broadcast[i] = network[i] | ^net.IP(ipnet.Mask)[i]
	}
	var count int64
	if hostBits >= 63 {
		count = -1
	} else if hostBits == 0 {
		count = 1
	} else {
		count = int64(1)<<uint(hostBits) - 2
		if count < 0 {
			count = 0
		}
	}
	// usable host range (network+1 .. broadcast-1)
	incIP(first)
	decIP(last)
	return SubnetInfo{
		CIDR:      cidr,
		Network:   network.String(),
		Mask:      mask,
		Broadcast: broadcast.String(),
		FirstHost: first.String(),
		LastHost:  last.String(),
		HostCount: count,
		Prefix:    ones,
	}, nil
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func decIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] > 0 {
			ip[i]--
			break
		}
		ip[i] = 0xff
	}
}

// RandomString returns a random string of length n from the given charset.
func RandomString(n int, charset string) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("length must be positive")
	}
	if charset == "" {
		charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}
	buf := make([]byte, n)
	randBytes := make([]byte, n)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = charset[int(randBytes[i])%len(charset)]
	}
	return string(buf), nil
}

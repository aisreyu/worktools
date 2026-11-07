package toolbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- port helpers ----

// ExpandPortSpec parses a port specification such as "80,443,8000-8010"
// into a de-duplicated, sorted list of ports. Whitespace is ignored.
func ExpandPortSpec(spec string) ([]int, error) {
	seen := map[int]bool{}
	for _, tok := range strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == ';'
	}) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(tok, "-"); ok {
			start, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return nil, fmt.Errorf("netscan_bad_range: %s", tok)
			}
			end, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return nil, fmt.Errorf("netscan_bad_range: %s", tok)
			}
			if start > end {
				start, end = end, start
			}
			if start < 1 || end > 65535 {
				return nil, fmt.Errorf("netscan_range_out: %s", tok)
			}
			if end-start > 4096 {
				return nil, fmt.Errorf("netscan_range_too_big: %s", tok)
			}
			for p := start; p <= end; p++ {
				seen[p] = true
			}
			continue
		}
		p, err := strconv.Atoi(tok)
		if err != nil {
			return nil, fmt.Errorf("netscan_bad_port: %s", tok)
		}
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("netscan_port_out: %s", tok)
		}
		seen[p] = true
	}
	out := make([]int, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}

// ---- HTTP probe (lightweight curl) ----

// HeaderKV is a single request header.
type HeaderKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// HTTPRequestOptions configures an HTTP probe.
type HTTPRequestOptions struct {
	URL             string     `json:"url"`
	Method          string     `json:"method"`
	Headers         []HeaderKV `json:"headers"`
	Body            string     `json:"body"`
	TimeoutMS       int        `json:"timeoutMs"`
	Retries         int        `json:"retries"`
	RetryDelayMS    int        `json:"retryDelayMs"`
	FollowRedirects *bool      `json:"followRedirects"`
	MaxBodyBytes    int        `json:"maxBodyBytes"`
}

// HTTPTimings breaks a request down into its network phases (ms).
type HTTPTimings struct {
	DNSMS   int64 `json:"dnsMs"`
	ConnMS  int64 `json:"connMs"`
	TLSMS   int64 `json:"tlsMs"`
	TTFBMS  int64 `json:"ttfbMs"`
	TotalMS int64 `json:"totalMs"`
}

// HTTPProbeResult is the outcome of an HTTP probe.
type HTTPProbeResult struct {
	Status      int               `json:"status"`
	StatusText  string            `json:"statusText"`
	Latency     string            `json:"latency"`
	LatencyMS   int64             `json:"latencyMs"`
	Proto       string            `json:"proto"`
	Headers     map[string]string `json:"headers"`
	BodySnippet string            `json:"bodySnippet"`
	BodySize    int               `json:"bodySize"`
	Attempts    int               `json:"attempts"`
	FinalURL    string            `json:"finalUrl"`
	Redirects   []string          `json:"redirects"`
	Timings     HTTPTimings       `json:"timings"`
}

// HTTPRequest issues an HTTP request and reports status, timings, headers and
// a body snippet. It supports arbitrary methods, headers, a body, redirect
// control and retries — a "lightweight curl".
func HTTPRequest(ctx context.Context, opts HTTPRequestOptions) (HTTPProbeResult, error) {
	rawURL := strings.TrimSpace(opts.URL)
	if rawURL == "" {
		return HTTPProbeResult{}, fmt.Errorf("netscan_url_empty")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return HTTPProbeResult{}, fmt.Errorf("netscan_bad_url: %s", err.Error())
	}
	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	if method == "" {
		method = http.MethodGet
	}
	if opts.TimeoutMS <= 0 {
		opts.TimeoutMS = 10000
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 4096
	}
	follow := true
	if opts.FollowRedirects != nil {
		follow = *opts.FollowRedirects
	}
	attempts := opts.Retries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 && opts.RetryDelayMS > 0 {
			select {
			case <-time.After(time.Duration(opts.RetryDelayMS) * time.Millisecond):
			case <-ctx.Done():
				return HTTPProbeResult{}, ctx.Err()
			}
		}
		res, err := doHTTPAttempt(ctx, rawURL, method, opts, follow)
		if err == nil {
			res.Attempts = attempt
			return res, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("netscan_request_failed")
	}
	return HTTPProbeResult{}, lastErr
}

func doHTTPAttempt(ctx context.Context, rawURL, method string, opts HTTPRequestOptions, follow bool) (HTTPProbeResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.TimeoutMS)*time.Millisecond)
	defer cancel()

	var bodyReader io.Reader
	if opts.Body != "" && method != http.MethodGet && method != http.MethodHead {
		bodyReader = bytes.NewBufferString(opts.Body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, bodyReader)
	if err != nil {
		return HTTPProbeResult{}, err
	}
	for _, h := range opts.Headers {
		k := strings.TrimSpace(h.Key)
		if k == "" {
			continue
		}
		req.Header.Set(k, h.Value)
	}

	var (
		dnsStart, dnsDone, connStart, tlsStart, wroteReq, firstByte time.Time
		redirects                                                    []string
	)
	trace := &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:              func(httptrace.DNSDoneInfo) { dnsDone = time.Now() },
		ConnectStart:         func(string, string) { connStart = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		WroteRequest:         func(httptrace.WroteRequestInfo) { wroteReq = time.Now() },
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}
	req = req.WithContext(httptrace.WithClientTrace(reqCtx, trace))

	client := &http.Client{Timeout: time.Duration(opts.TimeoutMS) * time.Millisecond}
	if !follow {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	} else {
		client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			redirects = append(redirects, r.URL.String())
			if len(via) >= 10 {
				return fmt.Errorf("netscan_too_many_redirects")
			}
			return nil
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return HTTPProbeResult{}, err
	}
	defer resp.Body.Close()
	firstByteAt := firstByte
	if firstByteAt.IsZero() {
		firstByteAt = time.Now()
	}

	limited := io.LimitReader(resp.Body, int64(opts.MaxBodyBytes)+1)
	raw, _ := io.ReadAll(limited)
	truncated := len(raw) > opts.MaxBodyBytes
	if truncated {
		raw = raw[:opts.MaxBodyBytes]
	}
	snippet := string(raw)
	if truncated {
		snippet += "\n...(truncated)"
	}
	total := time.Since(start)

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[k] = strings.Join(v, ", ")
	}

	timings := HTTPTimings{TotalMS: total.Milliseconds()}
	if !dnsStart.IsZero() && !dnsDone.IsZero() {
		timings.DNSMS = dnsDone.Sub(dnsStart).Milliseconds()
	}
	if !connStart.IsZero() {
		end := wroteReq
		if !tlsStart.IsZero() && tlsStart.After(connStart) {
			timings.TLSMS = firstByteAt.Sub(tlsStart).Milliseconds()
			timings.ConnMS = tlsStart.Sub(connStart).Milliseconds()
		} else if !end.IsZero() {
			timings.ConnMS = end.Sub(connStart).Milliseconds()
		}
	}
	if !wroteReq.IsZero() {
		timings.TTFBMS = firstByteAt.Sub(wroteReq).Milliseconds()
	}

	return HTTPProbeResult{
		Status:      resp.StatusCode,
		StatusText:  resp.Status,
		Latency:     total.Round(time.Millisecond).String(),
		LatencyMS:   total.Milliseconds(),
		Proto:       resp.Proto,
		Headers:     headers,
		BodySnippet: snippet,
		BodySize:    len(raw),
		FinalURL:    resp.Request.URL.String(),
		Redirects:   redirects,
		Timings:     timings,
	}, nil
}

// ---- TCPing ----
// TCPingSample is one probe result.
type TCPingSample struct {
	Seq       int     `json:"seq"`
	Open      bool    `json:"open"`
	LatencyMS float64 `json:"latencyMs"`
	Latency   string  `json:"latency"`
	Error     string  `json:"error,omitempty"`
}

// TCPingResult summarises a multi-sample TCP reachability probe.
type TCPingResult struct {
	Host        string         `json:"host"`
	Port        int            `json:"port"`
	Sent        int            `json:"sent"`
	Received    int            `json:"received"`
	LossPercent float64        `json:"lossPercent"`
	Min         string         `json:"min"`
	Avg         string         `json:"avg"`
	Max         string         `json:"max"`
	Samples     []TCPingSample `json:"samples"`
}

// TCPing probes host:port count times, measuring connect latency and packet
// loss — an ICMP-ping alternative that needs no special privileges.
func TCPing(ctx context.Context, host string, port, count, timeoutMS, intervalMS int) (TCPingResult, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return TCPingResult{}, fmt.Errorf("netscan_host_empty")
	}
	if port <= 0 {
		port = 80
	}
	if count <= 0 {
		count = 4
	}
	if count > 100 {
		count = 100
	}
	if timeoutMS <= 0 {
		timeoutMS = 2000
	}
	if intervalMS < 0 {
		intervalMS = 0
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: time.Duration(timeoutMS) * time.Millisecond}

	res := TCPingResult{Host: host, Port: port, Sent: count}
	for i := 1; i <= count; i++ {
		if i > 1 && intervalMS > 0 {
			select {
			case <-time.After(time.Duration(intervalMS) * time.Millisecond):
			case <-ctx.Done():
				return res, ctx.Err()
			}
		}
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		rtt := time.Since(start)
		s := TCPingSample{Seq: i, LatencyMS: float64(rtt.Microseconds()) / 1000.0, Latency: rtt.Round(time.Millisecond).String()}
		if err != nil {
			s.Error = err.Error()
		} else {
			s.Open = true
			res.Received++
			_ = conn.Close()
		}
		res.Samples = append(res.Samples, s)
	}
	if res.Sent > 0 {
		res.LossPercent = float64(res.Sent-res.Received) / float64(res.Sent) * 100
	}
	// Stats over successful samples.
	var ok []float64
	for _, s := range res.Samples {
		if s.Open {
			ok = append(ok, s.LatencyMS)
		}
	}
	if len(ok) > 0 {
		mn, mx, sum := ok[0], ok[0], 0.0
		for _, v := range ok {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
			sum += v
		}
		res.Min = fmt.Sprintf("%.1f ms", mn)
		res.Max = fmt.Sprintf("%.1f ms", mx)
		res.Avg = fmt.Sprintf("%.1f ms", sum/float64(len(ok)))
	}
	return res, nil
}

// ---- port presets ----

// PortPreset is a named group of well-known ports for one-click scanning.
type PortPreset struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Spec  string `json:"spec"`
	Count int    `json:"count"`
}

// PortPresets returns the built-in scan groups (web/db/middleware/etc.).
func PortPresets() []PortPreset {
	raw := []struct{ name, label, spec string }{
		{"web", "web", "80,443,8000,8080,8443,8888,9000,9080"},
		{"app", "app", "3000,5000,5173,7001,8000,8080,8081,9000"},
		{"db", "db", "1433,1521,3306,5432,6379,27017,9200,9300"},
		{"middleware", "middleware", "5672,6379,9092,11211,15672,1883,2181,61616,9090"},
		{"bigdata", "bigdata", "4040,7077,8000,8020,8032,8042,8088,9000,9083,9870,10000,16010,50070"},
		{"remote", "remote", "22,23,3389,5900,5985,5986,623,873"},
		{"container", "container", "2375,2376,2379,2380,6443,10250,10255,10257,10259"},
		{"monitor", "monitor", "3000,3100,8086,8123,9090,9093,9100,9121,9187,9273,9080"},
	}
	out := make([]PortPreset, 0, len(raw))
	for _, r := range raw {
		ports, _ := ExpandPortSpec(r.spec)
		out = append(out, PortPreset{Name: r.name, Label: r.label, Spec: r.spec, Count: len(ports)})
	}
	return out
}

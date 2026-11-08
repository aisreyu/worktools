package toolbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ipAPIBase is the geolocation endpoint. It is a package variable so tests can
// point it at a local server.
var ipAPIBase = "http://ip-api.com"

// ipAPIFields selects the response fields returned by ip-api.com.
const ipAPIFields = "status,message,query,country,countryCode,region,regionName,city,zip,lat,lon,timezone,isp,org,as"

// IPLookupResult is the geolocation result for a single IP or domain.
type IPLookupResult struct {
	Input       string  `json:"input"`
	OK          bool    `json:"ok"`
	Message     string  `json:"message"`
	Query       string  `json:"query"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	AS          string  `json:"as"`
}

// ipAPIRecord mirrors the raw ip-api.com response shape.
type ipAPIRecord struct {
	Status      string  `json:"status"`
	Message     string  `json:"message"`
	Query       string  `json:"query"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	AS          string  `json:"as"`
}

func (rec ipAPIRecord) toResult(input string) IPLookupResult {
	r := IPLookupResult{
		Input:       input,
		OK:          rec.Status == "success",
		Message:     rec.Message,
		Query:       rec.Query,
		Country:     rec.Country,
		CountryCode: rec.CountryCode,
		Region:      rec.RegionName,
		City:        rec.City,
		Zip:         rec.Zip,
		Lat:         rec.Lat,
		Lon:         rec.Lon,
		Timezone:    rec.Timezone,
		ISP:         rec.ISP,
		Org:         rec.Org,
		AS:          rec.AS,
	}
	if !r.OK && r.Message == "" {
		r.Message = "查询失败"
	}
	return r
}

// normalizeIPInput trims scheme, path and spaces from a user-supplied IP or
// domain so it can be sent to the lookup API.
func normalizeIPInput(q string) string {
	q = strings.TrimSpace(q)
	q = strings.TrimPrefix(q, "http://")
	q = strings.TrimPrefix(q, "https://")
	if i := strings.IndexAny(q, "/ "); i >= 0 {
		q = q[:i]
	}
	return strings.TrimSuffix(q, ".")
}

// resolveIPQuery turns a user input into an address the API understands. An
// empty input means "the caller's own public IP" and is passed through as "".
func resolveIPQuery(ctx context.Context, q string) (string, error) {
	if q == "" {
		return "", nil
	}
	if ip := net.ParseIP(strings.Trim(q, "[]")); ip != nil {
		return ip.String(), nil
	}
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupHost(rctx, q)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("无法解析 %q", q)
	}
	return ips[0], nil
}

// ipAPIRequest performs an HTTP call against the geolocation API and returns
// the raw body.
func ipAPIRequest(ctx context.Context, method, rawURL string, body []byte, timeoutMS int) ([]byte, error) {
	if timeoutMS <= 0 {
		timeoutMS = 8000
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(cctx, method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("接口返回 HTTP %d", resp.StatusCode)
	}
	return data, nil
}

// IPLookup geolocates a single IP or domain. An empty query looks up the
// caller's own public IP.
func IPLookup(ctx context.Context, query string, timeoutMS int) (IPLookupResult, error) {
	input := strings.TrimSpace(query)
	res := IPLookupResult{Input: input}
	ip, err := resolveIPQuery(ctx, normalizeIPInput(input))
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	rawURL := fmt.Sprintf("%s/json/%s?lang=zh-CN&fields=%s", ipAPIBase, url.PathEscape(ip), ipAPIFields)
	data, err := ipAPIRequest(ctx, http.MethodGet, rawURL, nil, timeoutMS)
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	var rec ipAPIRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		res.Message = "解析响应失败"
		return res, fmt.Errorf("解析响应失败: %w", err)
	}
	return rec.toResult(input), nil
}

// IPLookupBatch geolocates several IPs or domains at once (max 100 per request,
// extra inputs are chunked). The returned slice preserves input order; per-item
// failures carry their own Message instead of aborting the whole batch.
func IPLookupBatch(ctx context.Context, queries []string, timeoutMS int) ([]IPLookupResult, error) {
	type prepared struct {
		input string
		ip    string
		err   error
	}
	prep := make([]prepared, 0, len(queries))
	send := make([]int, 0, len(queries))
	for _, q := range queries {
		raw := strings.TrimSpace(q)
		norm := normalizeIPInput(raw)
		if norm == "" {
			prep = append(prep, prepared{input: raw, err: fmt.Errorf("请输入 IP 或域名")})
			continue
		}
		ip, err := resolveIPQuery(ctx, norm)
		prep = append(prep, prepared{input: raw, ip: ip, err: err})
		if err == nil {
			send = append(send, len(prep)-1)
		}
	}

	results := make([]IPLookupResult, len(prep))
	for i, p := range prep {
		if p.err != nil {
			results[i] = IPLookupResult{Input: p.input, Message: p.err.Error()}
		}
	}
	if len(send) == 0 {
		return results, nil
	}

	const chunkSize = 100
	for start := 0; start < len(send); start += chunkSize {
		end := start + chunkSize
		if end > len(send) {
			end = len(send)
		}
		idxs := send[start:end]
		ips := make([]string, len(idxs))
		for i, pi := range idxs {
			ips[i] = prep[pi].ip
		}
		body, _ := json.Marshal(ips)
		rawURL := fmt.Sprintf("%s/batch?lang=zh-CN&fields=%s", ipAPIBase, ipAPIFields)
		data, err := ipAPIRequest(ctx, http.MethodPost, rawURL, body, timeoutMS)
		if err != nil {
			for _, pi := range idxs {
				results[pi] = IPLookupResult{Input: prep[pi].input, Message: err.Error()}
			}
			continue
		}
		var recs []ipAPIRecord
		if err := json.Unmarshal(data, &recs); err != nil {
			for _, pi := range idxs {
				results[pi] = IPLookupResult{Input: prep[pi].input, Message: "解析响应失败"}
			}
			continue
		}
		for i, pi := range idxs {
			if i < len(recs) {
				results[pi] = recs[i].toResult(prep[pi].input)
			} else {
				results[pi] = IPLookupResult{Input: prep[pi].input, Message: "无结果"}
			}
		}
	}
	return results, nil
}

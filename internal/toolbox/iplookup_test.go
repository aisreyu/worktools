package toolbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeIPInput(t *testing.T) {
	cases := map[string]string{
		"  1.1.1.1 ":            "1.1.1.1",
		"http://1.1.1.1":        "1.1.1.1",
		"https://1.1.1.1/foo":   "1.1.1.1",
		"1.1.1.1/path":          "1.1.1.1",
		"example.com.":          "example.com",
		"":                      "",
	}
	for in, want := range cases {
		if got := normalizeIPInput(in); got != want {
			t.Errorf("normalizeIPInput(%q) = %q, want %q", in, got, want)
		}
	}
}

// withFakeAPI points ipAPIBase at a local server for the duration of the test.
func withFakeAPI(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := ipAPIBase
	ipAPIBase = srv.URL
	t.Cleanup(func() {
		ipAPIBase = old
		srv.Close()
	})
}

func TestIPLookupSingle(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/1.1.1.1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success", "query": "1.1.1.1", "country": "澳大利亚",
			"countryCode": "AU", "regionName": "Queensland", "city": "South Brisbane",
			"lat": -27.48, "lon": 153.02, "timezone": "Australia/Brisbane",
			"isp": "Cloudflare", "org": "APNIC", "as": "AS13335 Cloudflare",
		})
	})

	res, err := IPLookup(context.Background(), "1.1.1.1", 5000)
	if err != nil {
		t.Fatalf("IPLookup error: %v", err)
	}
	if !res.OK || res.CountryCode != "AU" || res.City != "South Brisbane" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Input != "1.1.1.1" || res.AS == "" {
		t.Errorf("missing fields: %+v", res)
	}
}

func TestIPLookupFailureStatus(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "fail", "message": "reserved range", "query": "10.0.0.1",
		})
	})

	res, err := IPLookup(context.Background(), "10.0.0.1", 5000)
	if err != nil {
		t.Fatalf("expected nil error for API-level failure, got %v", err)
	}
	if res.OK || res.Message != "reserved range" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestIPLookupHTTPError(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	if _, err := IPLookup(context.Background(), "1.1.1.1", 5000); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}

func TestIPLookupBatchOrderAndErrors(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/batch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var ips []string
		if err := json.Unmarshal(body, &ips); err != nil {
			t.Fatalf("bad request body: %v", err)
		}
		// Echo back one success record per sent IP.
		out := make([]map[string]any, len(ips))
		for i, ip := range ips {
			out[i] = map[string]any{"status": "success", "query": ip, "country": "测试", "countryCode": "ZZ"}
		}
		_ = json.NewEncoder(w).Encode(out)
	})

	// Include a blank entry (own error) and an unresolvable hostname in between.
	queries := []string{"8.8.8.8", "", "this-host-does-not-exist.invalid", "9.9.9.9"}
	res, err := IPLookupBatch(context.Background(), queries, 8000)
	if err != nil {
		t.Fatalf("IPLookupBatch error: %v", err)
	}
	if len(res) != len(queries) {
		t.Fatalf("expected %d results, got %d", len(queries), len(res))
	}
	if res[0].Query != "8.8.8.8" || !res[0].OK {
		t.Errorf("result[0] unexpected: %+v", res[0])
	}
	if res[1].OK || res[1].Message == "" {
		t.Errorf("result[1] should be a local error: %+v", res[1])
	}
	if res[2].OK || res[2].Message == "" {
		t.Errorf("result[2] should be a resolution error: %+v", res[2])
	}
	if res[3].Query != "9.9.9.9" || !res[3].OK {
		t.Errorf("result[3] unexpected (order not preserved?): %+v", res[3])
	}
}

// TestIPLookupSmoke hits the real API; skipped when offline.
func TestIPLookupSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := IPLookup(ctx, "114.114.114.114", 8000)
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if !res.OK || res.Country == "" {
		t.Fatalf("unexpected smoke result: %+v", res)
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	t.Log(string(b))
}

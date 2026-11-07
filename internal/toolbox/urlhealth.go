package toolbox

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// URLHealthItem is one URL to probe.
type URLHealthItem struct {
	ID   int64  `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// URLHealthResult is the outcome of probing one URL.
type URLHealthResult struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	OK           bool   `json:"ok"`
	Status       int    `json:"status"`
	Latency      string `json:"latency"`
	Error        string `json:"error,omitempty"`
	CertDaysLeft int    `json:"certDaysLeft,omitempty"`
	CertExpiring bool   `json:"certExpiring,omitempty"`
	CertExpired  bool   `json:"certExpired,omitempty"`
}

// URLHealthCheck probes every URL concurrently for reachability and (for
// https URLs) certificate validity. Concurrency is capped and each probe is
// time-boxed so a large list still completes quickly.
func URLHealthCheck(ctx context.Context, items []URLHealthItem, timeoutMS int) []URLHealthResult {
	if timeoutMS <= 0 {
		timeoutMS = 8000
	}
	out := make([]URLHealthResult, len(items))
	const maxParallel = 8
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		go func(i int, it URLHealthItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = probeURLHealth(ctx, it, timeoutMS)
		}(i, it)
	}
	wg.Wait()
	return out
}

func probeURLHealth(ctx context.Context, it URLHealthItem, timeoutMS int) URLHealthResult {
	r := URLHealthResult{ID: it.ID, Name: it.Name, URL: it.URL}
	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := HTTPProbe(probeCtx, it.URL, timeoutMS)
	r.Latency = time.Since(start).Round(time.Millisecond).String()
	if err != nil {
		r.Error = err.Error()
	} else {
		r.OK = res.Status > 0 && res.Status < 500
		r.Status = res.Status
	}

	if u, uerr := url.Parse(it.URL); uerr == nil && u.Scheme == "https" {
		host := u.Hostname()
		port := 443
		if p := u.Port(); p != "" {
			fmt.Sscanf(p, "%d", &port)
		}
		if ssl, serr := SSLCheck(probeCtx, host, port, timeoutMS); serr == nil {
			r.CertDaysLeft = ssl.DaysRemaining
			r.CertExpiring = ssl.ExpiringSoon
			r.CertExpired = ssl.Expired
		}
	}
	return r
}

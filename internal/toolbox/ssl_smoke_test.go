package toolbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestSSLCheckSmoke hits a real site; skipped when offline.
func TestSSLCheckSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := SSLCheck(ctx, "www.baidu.com", 443, 8000)
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if res.TLSVersion == "" || len(res.Chain) == 0 {
		t.Fatalf("unexpected empty result: %+v", res)
	}
	if res.Leaf.NotAfter == "" || res.DaysRemaining == 0 {
		t.Errorf("leaf cert info missing: %+v", res.Leaf)
	}
	if res.Score <= 0 || res.Grade == "" {
		t.Errorf("score/grade missing: %d %q", res.Score, res.Grade)
	}
	found12 := false
	for _, p := range res.Protocols {
		if p.Version == "TLS 1.2" && p.Supported {
			found12 = true
		}
	}
	if !found12 {
		t.Errorf("baidu should support TLS 1.2, got %+v", res.Protocols)
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	t.Log(string(b))
}

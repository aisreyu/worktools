package cloudsync

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// fakeS3 is a tiny in-memory S3 that accepts our SigV4 requests and records
// them, so we can exercise Put/Get/List/Delete end to end without real creds.
type fakeS3 struct {
	srv    *httptest.Server
	bucket string
	objs   map[string][]byte
	times  map[string]time.Time
	authed []string // raw Authorization headers seen
}

func newFakeS3(t *testing.T) *fakeS3 {
	t.Helper()
	f := &fakeS3{bucket: "bkt", objs: map[string][]byte{}, times: map[string]time.Time{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/bkt/", func(w http.ResponseWriter, r *http.Request) {
		f.authed = append(f.authed, r.Header.Get("Authorization"))
		key := strings.TrimPrefix(r.URL.Path, "/bkt/")
		switch r.Method {
		case http.MethodPut:
			f.objs[key] = mustReadAll(t, r)
			f.times[key] = time.Now().UTC()
			w.WriteHeader(200)
		case http.MethodGet:
			if key == "" { // list
				var sb strings.Builder
				sb.WriteString("<?xml version=\"1.0\"?><ListBucketResult>")
				keys := make([]string, 0, len(f.objs))
				for k := range f.objs {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					sb.WriteString("<Contents><Key>" + k + "</Key><Size>" +
						itoa(len(f.objs[k])) + "</Size><LastModified>" +
						f.times[k].Format(time.RFC3339) + "</LastModified></Contents>")
				}
				sb.WriteString("</ListBucketResult>")
				_, _ = w.Write([]byte(sb.String()))
				return
			}
			data, ok := f.objs[key]
			if !ok {
				w.WriteHeader(404)
				_, _ = w.Write([]byte("<Error><Code>NoSuchKey</Code><Message>nope</Message></Error>"))
				return
			}
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(f.objs, key)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func mustReadAll(t *testing.T, r *http.Request) []byte {
	t.Helper()
	b := make([]byte, 0, 1024)
	buf := make([]byte, 512)
	for {
		n, err := r.Body.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil {
			break
		}
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func testCfg(f *fakeS3) Config {
	return Config{
		Endpoint:  f.srv.URL,
		Region:    "us-east-1",
		Bucket:    f.bucket,
		Prefix:    "backups/",
		PathStyle: true,
		AccessKey: "AKIAEXAMPLE",
		SecretKey: "secret",
	}
}

func TestPutGetListDelete(t *testing.T) {
	f := newFakeS3(t)
	c := New(testCfg(f))

	key := JoinPrefix("backups", "worktools-autobackup-20260101-010101.json")
	if err := c.Put(key, []byte(`{"hello":"world"}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := c.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != `{"hello":"world"}` {
		t.Fatalf("Get roundtrip mismatch: %s", got)
	}
	objs, err := c.List("backups/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objs) != 1 || objs[0].Key != key || objs[0].Size != int64(len(`{"hello":"world"}`)) {
		t.Fatalf("List mismatch: %+v", objs)
	}
	if err := c.Delete(key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := c.Get(key); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	// Delete again is idempotent.
	if err := c.Delete(key); err != nil {
		t.Fatalf("idempotent Delete: %v", err)
	}
}

func TestHeadAndAuth(t *testing.T) {
	f := newFakeS3(t)
	c := New(testCfg(f))
	if err := c.Head(); err != nil {
		t.Fatalf("Head: %v", err)
	}
	if len(f.authed) == 0 {
		t.Fatal("no Authorization header captured")
	}
	auth := f.authed[0]
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIAEXAMPLE/") {
		t.Fatalf("unexpected Authorization: %s", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=host;x-amz-content;x-amz-date") {
		t.Fatalf("signed headers missing: %s", auth)
	}
}

func TestJoinPrefix(t *testing.T) {
	if got := JoinPrefix("", "a.json"); got != "a.json" {
		t.Fatalf("empty prefix: %q", got)
	}
	if got := JoinPrefix("/backups/", "a.json"); got != "backups/a.json" {
		t.Fatalf("slash trim: %q", got)
	}
}

func TestInvalidEndpoint(t *testing.T) {
	c := New(Config{Endpoint: "not a url", Bucket: "b"})
	if err := c.Head(); err == nil {
		t.Fatal("expected error for invalid endpoint")
	}
}

// TestRealS3 is skipped unless WORKTOOLS_S3_TEST points at a real bucket; it
// exercises the signer against a live store (MinIO: `minio server /tmp/mtest`).
func TestRealS3(t *testing.T) {
	endpoint := os.Getenv("WORKTOOLS_S3_TEST")
	if endpoint == "" {
		t.Skip("WORKTOOLS_S3_TEST not set")
	}
	cfg := Config{
		Endpoint:  endpoint,
		Region:    os.Getenv("WORKTOOLS_S3_REGION"),
		Bucket:    os.Getenv("WORKTOOLS_S3_BUCKET"),
		Prefix:    "worktools-test/",
		PathStyle: true,
		AccessKey: os.Getenv("WORKTOOLS_S3_AK"),
		SecretKey: os.Getenv("WORKTOOLS_S3_SK"),
	}
	if cfg.Bucket == "" || cfg.AccessKey == "" {
		t.Skip("bucket/creds not set")
	}
	c := New(cfg)
	if err := c.Head(); err != nil {
		t.Fatalf("Head: %v", err)
	}
	key := JoinPrefix(cfg.Prefix, "roundtrip-test.json")
	if err := c.Put(key, []byte("ping")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	defer c.Delete(key)
	got, err := c.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "ping" {
		t.Fatalf("roundtrip: %q", got)
	}
}

var _ = filepath.Join // keep import if unused in trimmed builds

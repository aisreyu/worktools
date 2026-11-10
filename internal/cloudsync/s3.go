// Package cloudsync uploads and lists encrypted backup files on S3-compatible
// object storage (MinIO, AWS S3, Aliyun OSS via S3 gateway, Tencent COS,
// Cloudflare R2, ...). It speaks plain HTTPS so it stays pure Go and cross-
// compilable; credentials never leave this package except in the requests
// themselves.
package cloudsync

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Config describes one S3-compatible target. Endpoint is a full URL such as
// https://s3.amazonaws.com or https://play.min.io; PathStyle should be true
// for MinIO and most on-prem stores.
type Config struct {
	Endpoint   string // e.g. https://s3.cn-north-1.amazonaws.com.cn
	Region     string
	Bucket     string
	Prefix     string // object key prefix, e.g. "worktools/backups/"
	PathStyle  bool
	AccessKey  string
	SecretKey  string
	SessionTok string // optional STS token
}

// Object is one remote backup object.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// ErrNotFound is returned when the bucket or object does not exist.
var ErrNotFound = errors.New("not found")

// client signs requests with AWS Signature V4. Only the S3 operations we
// need are implemented: PutObject, GetObject, DeleteObject, ListObjectsV2.
type client struct {
	cfg  Config
	http *http.Client
}

// New returns a client for cfg with a sane timeout.
func New(cfg Config) *client {
	return &client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}
}

// ---- object key helpers ----

// JoinPrefix normalises the prefix so keys look like "<prefix><name>".
func JoinPrefix(prefix, name string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func (c *client) bucketHost() (scheme, host, path string, err error) {
	u, err := url.Parse(strings.TrimSpace(c.cfg.Endpoint))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", "", fmt.Errorf("invalid endpoint %q", c.cfg.Endpoint)
	}
	scheme = u.Scheme
	host = u.Host
	if c.cfg.PathStyle {
		path = "/" + c.cfg.Bucket
	} else {
		host = c.cfg.Bucket + "." + u.Host
		path = ""
	}
	return scheme, host, path, nil
}

// ---- AWS Signature V4 ----

const (
	amzDateFmt = "20060102T150405Z"
	shortDate  = "20060102"
	emptySHA   = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	alg        = "AWS4-HMAC-SHA256"
)

func hmacSHA(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func (c *client) signingKey(secret, date, region string) []byte {
	kDate := hmacSHA([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA(kDate, []byte(region))
	kService := hmacSHA(kRegion, []byte("s3"))
	return hmacSHA(kService, []byte("aws4_request"))
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// do signs and performs one S3 request. rawPath is the object key path
// (already including the bucket path segment for path-style addressing).
// query is the raw query string without the leading '?'. body may be nil.
func (c *client) do(method, rawPath, query string, body []byte, extraHeaders map[string]string) (int, []byte, error) {
	scheme, host, _, err := c.bucketHost()
	if err != nil {
		return 0, nil, err
	}
	region := c.cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	now := time.Now().UTC()
	amzDate := now.Format(amzDateFmt)
	date := now.Format(shortDate)

	payloadHash := emptySHA
	if len(body) > 0 {
		payloadHash = sha256Hex(body)
	}

	// Canonical URI must be URI-encoded per S3 rules (each path segment
	// encoded, '/' kept). Object names are simple timestamps so a light
	// encode suffices.
	canonicalURI := encodePath(rawPath)
	canonicalQuery := canonicalQueryString(query)

	headers := map[string]string{
		"host":          host,
		"x-amz-content": payloadHash,
		"x-amz-date":    amzDate,
	}
	if c.cfg.SessionTok != "" {
		headers["x-amz-security-token"] = c.cfg.SessionTok
	}
	for k, v := range extraHeaders {
		headers[strings.ToLower(k)] = v
	}

	// Canonical headers: sorted lowercase names.
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var chBuf strings.Builder
	for _, k := range names {
		chBuf.WriteString(k + ":" + strings.TrimSpace(headers[k]) + "\n")
	}
	canonicalHeaders := chBuf.String()
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := date + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		alg,
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	sig := hex.EncodeToString(hmacSHA(c.signingKey(c.cfg.SecretKey, date, region), []byte(stringToSign)))

	uq := ""
	if canonicalQuery != "" {
		uq = "?" + query
	}
	reqURL := scheme + "://" + host + canonicalURI + uq
	req, err := http.NewRequest(method, reqURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Host = host
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if c.cfg.SessionTok != "" {
		req.Header.Set("x-amz-security-token", c.cfg.SessionTok)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", alg+" Credential="+c.cfg.AccessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+sig)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

// encodePath URI-encodes each segment of p, keeping slashes.
func encodePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// canonicalQueryString sorts query params (already URL-encoded) by name.
func canonicalQueryString(query string) string {
	if query == "" {
		return ""
	}
	parts := strings.Split(query, "&")
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

// ---- S3 operations ----

// objPath builds the full request path for an object key: path-style keeps
// the bucket in the path, virtual-host style addresses the bucket via the
// Host header so the path is just the key.
func (c *client) objPath(key string) (string, error) {
	_, _, base, err := c.bucketHost()
	if err != nil {
		return "", err
	}
	if base == "" {
		return "/" + encodePath(key), nil
	}
	return base + "/" + encodePath(key), nil
}

// Put stores data under key.
func (c *client) Put(key string, data []byte) error {
	p, err := c.objPath(key)
	if err != nil {
		return err
	}
	code, body, err := c.do("PUT", p, "", data, map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		return err
	}
	if code >= 300 {
		return apiErr("PUT", key, code, body)
	}
	return nil
}

// Get fetches the object under key.
func (c *client) Get(key string) ([]byte, error) {
	p, err := c.objPath(key)
	if err != nil {
		return nil, err
	}
	code, body, err := c.do("GET", p, "", nil, nil)
	if err != nil {
		return nil, err
	}
	switch {
	case code == 200:
		return body, nil
	case code == 404:
		return nil, ErrNotFound
	default:
		return nil, apiErr("GET", key, code, body)
	}
}

// Delete removes the object under key (idempotent).
func (c *client) Delete(key string) error {
	p, err := c.objPath(key)
	if err != nil {
		return err
	}
	code, body, err := c.do("DELETE", p, "", nil, nil)
	if err != nil {
		return err
	}
	if code >= 300 && code != 404 {
		return apiErr("DELETE", key, code, body)
	}
	return nil
}

// listBucket is the XML shape of a ListObjectsV2 response.
type listBucket struct {
	Contents []struct {
		Key          string `xml:"Key"`
		Size         int64  `xml:"Size"`
		LastModified string `xml:"LastModified"`
	} `xml:"Contents"`
}

// List returns the objects under prefix, newest first.
func (c *client) List(prefix string) ([]Object, error) {
	_, _, base, err := c.bucketHost()
	if err != nil {
		return nil, err
	}
	q := "list-type=2&prefix=" + url.QueryEscape(prefix)
	code, body, err := c.do("GET", base+"/", q, nil, nil)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, apiErr("LIST", prefix, code, body)
	}
	var lb listBucket
	if err := xml.Unmarshal(body, &lb); err != nil {
		return nil, fmt.Errorf("parse list response: %w", err)
	}
	out := make([]Object, 0, len(lb.Contents))
	for _, o := range lb.Contents {
		t, _ := time.Parse(time.RFC3339, o.LastModified)
		out = append(out, Object{Key: o.Key, Size: o.Size, LastModified: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastModified.After(out[j].LastModified) })
	return out, nil
}

// Head checks bucket reachability with a signed list request (max-keys=1).
func (c *client) Head() error {
	_, _, base, err := c.bucketHost()
	if err != nil {
		return err
	}
	code, body, err := c.do("GET", base+"/", "list-type=2&max-keys=1", nil, nil)
	if err != nil {
		return err
	}
	if code >= 300 {
		return apiErr("HEAD", c.cfg.Bucket, code, body)
	}
	return nil
}

// apiErr converts an S3 XML error document into a readable error.
func apiErr(op, target string, code int, body []byte) error {
	var xe struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if err := xml.Unmarshal(body, &xe); err == nil && xe.Message != "" {
		return fmt.Errorf("s3 %s %s: HTTP %d: %s: %s", op, target, code, xe.Code, xe.Message)
	}
	return fmt.Errorf("s3 %s %s: HTTP %d", op, target, code)
}

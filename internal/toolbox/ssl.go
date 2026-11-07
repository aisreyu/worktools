package toolbox

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SSLCheckResult is the outcome of an SSL/TLS certificate inspection,
// modelled loosely on SSL Labs' report: certificate chain, validity,
// protocol versions and cipher suites offered by the server.
type SSLCheckResult struct {
	Host          string          `json:"host"`
	Port          int             `json:"port"`
	IP            string          `json:"ip"`
	TLSVersion    string          `json:"tlsVersion"`
	CipherSuite   string          `json:"cipherSuite"`
	Latency       string          `json:"latency"`
	Chain         []CertInfo      `json:"chain"`
	Leaf          CertInfo        `json:"leaf"`
	DaysRemaining int             `json:"daysRemaining"`
	Expired       bool            `json:"expired"`
	ExpiringSoon  bool            `json:"expiringSoon"`
	Protocols     []ProtocolInfo  `json:"protocols"`
	Issues        []string        `json:"issues"`
	Score         int             `json:"score"`
	Grade         string          `json:"grade"`
}

// CertInfo describes one certificate in the chain.
type CertInfo struct {
	Subject             string   `json:"subject"`
	Issuer              string   `json:"issuer"`
	SerialNumber        string   `json:"serialNumber"`
	NotBefore           string   `json:"notBefore"`
	NotAfter            string   `json:"notAfter"`
	DNSNames            []string `json:"dnsNames"`
	SignatureAlgorithm  string   `json:"signatureAlgorithm"`
	PublicKeyAlgorithm  string   `json:"publicKeyAlgorithm"`
	PublicKeyBits       int      `json:"publicKeyBits"`
	IsCA                bool     `json:"isCA"`
	SelfSigned          bool     `json:"selfSigned"`
	SubjectAltNameCount int      `json:"subjectAltNameCount"`
}

// ProtocolInfo is the outcome of probing one TLS version.
type ProtocolInfo struct {
	Version    string `json:"version"`
	Supported  bool   `json:"supported"`
	CipherName string `json:"cipherName,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SSLCheck connects to host:port, inspects the served certificate chain and
// probes which TLS protocol versions the server accepts.
func SSLCheck(ctx context.Context, host string, port int, timeoutMS int) (SSLCheckResult, error) {
	if port <= 0 {
		port = 443
	}
	if timeoutMS <= 0 {
		timeoutMS = 8000
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return SSLCheckResult{}, fmt.Errorf("host is required")
	}
	// Accept "host:port" input too.
	if h, p, err := net.SplitHostPort(host); err == nil {
		if n, err := strconv.Atoi(p); err == nil {
			host, port = h, n
		}
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: time.Duration(timeoutMS) * time.Millisecond}

	// Resolve for the report.
	var ipStr string
	if addrs, err := net.DefaultResolver.LookupHost(ctx, host); err == nil && len(addrs) > 0 {
		sort.Strings(addrs)
		ipStr = addrs[0]
	}

	// Main handshake: negotiate the best mutually supported version.
	cfg := &tls.Config{ServerName: host}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, cfg)
	if err != nil {
		return SSLCheckResult{}, fmt.Errorf("tls_handshake_failed: %s", err.Error())
	}
	defer conn.Close()
	latency := time.Since(start).Round(time.Millisecond).String()

	state := conn.ConnectionState()
	res := SSLCheckResult{
		Host:        host,
		Port:        port,
		IP:          ipStr,
		TLSVersion:  tlsVersionName(state.Version),
		CipherSuite: tls.CipherSuiteName(state.CipherSuite),
		Latency:     latency,
	}

	certs := state.PeerCertificates
	if len(certs) == 0 {
		return res, fmt.Errorf("no_certificate")
	}
	res.Chain = make([]CertInfo, 0, len(certs))
	for _, c := range certs {
		res.Chain = append(res.Chain, describeCert(c))
	}
	res.Leaf = res.Chain[0]

	leaf := certs[0]
	days := int(time.Until(leaf.NotAfter).Hours() / 24)
	res.DaysRemaining = days
	res.Expired = days < 0
	res.ExpiringSoon = days >= 0 && days <= 30

	// Hostname match.
	if err := leaf.VerifyHostname(host); err != nil {
		res.Issues = append(res.Issues, "hostname_mismatch: "+err.Error())
	}
	if res.Expired {
		res.Issues = append(res.Issues, fmt.Sprintf("expired:%d", -days))
	} else if res.ExpiringSoon {
		res.Issues = append(res.Issues, fmt.Sprintf("expiring:%d", days))
	}
	if len(certs) == 1 && leaf.CheckSignatureFrom(leaf) == nil {
		res.Issues = append(res.Issues, "self_signed_chain")
	}
	if strings.Contains(strings.ToLower(leaf.SignatureAlgorithm.String()), "sha1") {
		res.Issues = append(res.Issues, "sha1_signature")
	}
	if leaf.PublicKeyAlgorithm == x509.RSA && leaf.PublicKey.(*rsa.PublicKey).N.BitLen() < 2048 {
		res.Issues = append(res.Issues, "weak_rsa_key")
	}

	// Probe protocol versions (SSL 3.0 / TLS 1.0 / 1.1 are insecure).
	res.Protocols = probeProtocols(ctx, dialer, addr, host)

	for _, p := range res.Protocols {
		if !p.Supported {
			continue
		}
		switch p.Version {
		case "SSL 3.0":
			res.Issues = append(res.Issues, "supports_ssl3")
		case "TLS 1.0":
			res.Issues = append(res.Issues, "supports_tls10")
		case "TLS 1.1":
			res.Issues = append(res.Issues, "supports_tls11")
		}
	}

	res.Score, res.Grade = scoreSSL(res)
	return res, nil
}

// probeProtocols tries each TLS version with a fixed max, in parallel.
func probeProtocols(ctx context.Context, dialer *net.Dialer, addr, host string) []ProtocolInfo {
	versions := []struct {
		name  string
		value uint16
	}{
		{"SSL 3.0", tls.VersionSSL30},
		{"TLS 1.0", tls.VersionTLS10},
		{"TLS 1.1", tls.VersionTLS11},
		{"TLS 1.2", tls.VersionTLS12},
		{"TLS 1.3", tls.VersionTLS13},
	}
	out := make([]ProtocolInfo, len(versions))
	type result struct{ idx int; info ProtocolInfo }
	ch := make(chan result, len(versions))
	for i, v := range versions {
		go func(idx int, name string, ver uint16) {
			info := ProtocolInfo{Version: name}
			cfg := &tls.Config{
				ServerName:         host,
				MinVersion:         ver,
				MaxVersion:         ver,
				InsecureSkipVerify: true, // we only care about protocol support
			}
			d := &net.Dialer{Timeout: dialer.Timeout}
			conn, err := tls.DialWithDialer(d, "tcp", addr, cfg)
			if err != nil {
				info.Error = strings.SplitN(err.Error(), ": ", 2)[0]
			} else {
				info.Supported = true
				info.CipherName = tls.CipherSuiteName(conn.ConnectionState().CipherSuite)
				_ = conn.Close()
			}
			ch <- result{idx, info}
		}(i, v.name, v.value)
	}
	for range versions {
		r := <-ch
		out[r.idx] = r.info
	}
	return out
}

// describeCert converts an x509 certificate into the JSON-friendly struct.
func describeCert(c *x509.Certificate) CertInfo {
	info := CertInfo{
		Subject:             c.Subject.String(),
		Issuer:              c.Issuer.String(),
		SerialNumber:        c.SerialNumber.String(),
		NotBefore:           c.NotBefore.Local().Format("2006-01-02 15:04:05"),
		NotAfter:            c.NotAfter.Local().Format("2006-01-02 15:04:05"),
		DNSNames:            c.DNSNames,
		SignatureAlgorithm:  c.SignatureAlgorithm.String(),
		PublicKeyAlgorithm:  c.PublicKeyAlgorithm.String(),
		IsCA:                c.IsCA,
		SelfSigned:          bytesEqual(c.RawSubject, c.RawIssuer),
		SubjectAltNameCount: len(c.DNSNames),
	}
	switch pub := c.PublicKey.(type) {
	case *rsa.PublicKey:
		info.PublicKeyBits = pub.N.BitLen()
	case *ecdsa.PublicKey:
		info.PublicKeyBits = pub.Curve.Params().BitSize
	case ed25519.PublicKey:
		info.PublicKeyBits = 256
	}
	return info
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// tlsVersionName maps the TLS uint16 to a human name.
func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionSSL30:
		return "SSL 3.0"
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("unknown (0x%04x)", v)
	}
}

// scoreSSL computes a 0-100 score and letter grade à la SSL Labs.
func scoreSSL(res SSLCheckResult) (int, string) {
	score := 100
	for _, p := range res.Protocols {
		if !p.Supported {
			continue
		}
		switch p.Version {
		case "SSL 3.0":
			score -= 40
		case "TLS 1.0":
			score -= 25
		case "TLS 1.1":
			score -= 15
		}
	}
	if res.Expired {
		score -= 40
	} else if res.ExpiringSoon {
		score -= 10
	}
	for _, issue := range res.Issues {
		switch {
		case strings.HasPrefix(issue, "hostname_mismatch"):
			score -= 30
		case issue == "self_signed_chain":
			score -= 10
		case issue == "sha1_signature":
			score -= 20
		case issue == "weak_rsa_key":
			score -= 20
		}
	}
	if score < 0 {
		score = 0
	}
	return score, gradeOf(score)
}

func gradeOf(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	case score >= 50:
		return "E"
	default:
		return "F"
	}
}

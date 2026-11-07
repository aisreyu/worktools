package toolbox

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// CertGenOptions configures self-signed certificate generation.
type CertGenOptions struct {
	CommonName   string   `json:"commonName"`
	Organization string   `json:"organization"`
	OrgUnit      string   `json:"orgUnit"`
	Country      string   `json:"country"`
	DNSNames     []string `json:"dnsNames"`
	IPs          []string `json:"ips"`
	Days         int      `json:"days"`
	KeyType      string   `json:"keyType"` // rsa2048 | rsa4096 | ecdsa256 | ecdsa384 | ed25519
	IsCA         bool     `json:"isCa"`
}

// CertGenResult contains a generated certificate and private key in PEM form.
type CertGenResult struct {
	CertPEM   string   `json:"certPem"`
	KeyPEM    string   `json:"keyPem"`
	Subject   string   `json:"subject"`
	NotBefore string   `json:"notBefore"`
	NotAfter  string   `json:"notAfter"`
	KeyType   string   `json:"keyType"`
	SANs      []string `json:"sans"`
	Serial    string   `json:"serial"`
}

type generatedKey struct {
	priv    any
	pub     any
	keyType string
	signer  crypto.Signer
}

func generateKey(keyType string) (generatedKey, error) {
	switch strings.ToLower(strings.TrimSpace(keyType)) {
	case "", "rsa2048", "rsa":
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return generatedKey{}, err
		}
		return generatedKey{priv: k, pub: &k.PublicKey, keyType: "RSA-2048", signer: k}, nil
	case "rsa4096":
		k, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return generatedKey{}, err
		}
		return generatedKey{priv: k, pub: &k.PublicKey, keyType: "RSA-4096", signer: k}, nil
	case "ecdsa256", "ecdsa":
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return generatedKey{}, err
		}
		return generatedKey{priv: k, pub: &k.PublicKey, keyType: "ECDSA-P256", signer: k}, nil
	case "ecdsa384":
		k, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		if err != nil {
			return generatedKey{}, err
		}
		return generatedKey{priv: k, pub: &k.PublicKey, keyType: "ECDSA-P384", signer: k}, nil
	default:
		return generatedKey{}, fmt.Errorf("certgen_bad_key_type: %s", keyType)
	}
}

func buildTemplate(opts CertGenOptions, serial *big.Int, notBefore, notAfter time.Time) (*x509.Certificate, []string, error) {
	cn := strings.TrimSpace(opts.CommonName)
	if cn == "" && len(opts.DNSNames) == 0 && len(opts.IPs) == 0 {
		return nil, nil, fmt.Errorf("certgen_need_cn_or_san")
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: splitNonEmpty(opts.Organization),
			OrganizationalUnit: splitNonEmpty(opts.OrgUnit),
			Country:      splitNonEmpty(opts.Country),
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  opts.IsCA,
	}
	sans := []string{}
	for _, d := range opts.DNSNames {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		tmpl.DNSNames = append(tmpl.DNSNames, d)
		sans = append(sans, "DNS:"+d)
	}
	for _, ip := range opts.IPs {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			return nil, nil, fmt.Errorf("certgen_bad_ip: %s", ip)
		}
		tmpl.IPAddresses = append(tmpl.IPAddresses, parsed)
		sans = append(sans, "IP:"+ip)
	}
	if opts.IsCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature
	} else {
		tmpl.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	}
	if cn != "" && len(tmpl.DNSNames) == 0 && len(tmpl.IPAddresses) == 0 {
		// CN-only cert: add CN as a DNS SAN for modern clients.
		tmpl.DNSNames = append(tmpl.DNSNames, cn)
		sans = append(sans, "DNS:"+cn)
	}
	return tmpl, sans, nil
}

// GenerateSelfSignedCert creates a self-signed certificate and key (PEM).
func GenerateSelfSignedCert(opts CertGenOptions) (CertGenResult, error) {
	key, err := generateKey(opts.KeyType)
	if err != nil {
		return CertGenResult{}, err
	}
	days := opts.Days
	if days <= 0 {
		days = 365
	}
	if days > 3650 {
		days = 3650
	}
	notBefore := time.Now().Add(-time.Minute)
	notAfter := notBefore.Add(time.Duration(days) * 24 * time.Hour)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return CertGenResult{}, err
	}
	tmpl, sans, err := buildTemplate(opts, serial, notBefore, notAfter)
	if err != nil {
		return CertGenResult{}, err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.pub, key.signer)
	if err != nil {
		return CertGenResult{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key.priv)
	if err != nil {
		return CertGenResult{}, err
	}
	return CertGenResult{
		CertPEM:   string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		KeyPEM:    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
		Subject:   tmpl.Subject.String(),
		NotBefore: notBefore.Format(time.RFC3339),
		NotAfter:  notAfter.Format(time.RFC3339),
		KeyType:   key.keyType,
		SANs:      sans,
		Serial:    serial.Text(16),
	}, nil
}

// GenerateCSR creates a certificate signing request (PEM) with a fresh key.
func GenerateCSR(opts CertGenOptions) (CertGenResult, error) {
	key, err := generateKey(opts.KeyType)
	if err != nil {
		return CertGenResult{}, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl, sans, err := buildTemplate(opts, serial, time.Now(), time.Now().Add(365*24*time.Hour))
	if err != nil {
		return CertGenResult{}, err
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            tmpl.Subject,
		DNSNames:           tmpl.DNSNames,
		IPAddresses:        tmpl.IPAddresses,
		SignatureAlgorithm: 0,
	}, key.priv)
	if err != nil {
		return CertGenResult{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key.priv)
	if err != nil {
		return CertGenResult{}, err
	}
	return CertGenResult{
		CertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})),
		KeyPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
		Subject: tmpl.Subject.String(),
		KeyType: key.keyType,
		SANs:    sans,
		Serial:  serial.Text(16),
	}, nil
}

// CertPEMToInfo parses a PEM certificate so a freshly generated cert can be
// round-tripped through the same inspection view as the SSL checker.
func CertPEMToInfo(certPEM string) (CertInfo, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return CertInfo{}, fmt.Errorf("certgen_bad_pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return CertInfo{}, err
	}
	return describeCert(cert), nil
}

func splitNonEmpty(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []string{s}
}

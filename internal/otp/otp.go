// Package otp wraps TOTP/HOTP generation and otpauth:// URI parsing.
package otp

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"

	"github.com/aisreyu/worktools/internal/store"
)

// Params is the normalised set of options for generating a code.
type Params struct {
	Secret    string
	Algorithm string // SHA1 | SHA256 | SHA512
	Digits    int
	Period    int
	Counter   int64
	Type      string // totp | hotp
}

// Code is the result of a generation request.
type Code struct {
	Code      string `json:"code"`
	Remaining int    `json:"remaining"` // seconds left in the current window (TOTP)
	Period    int    `json:"period"`
}

// algo maps a friendly name to pquerna's algorithm enum.
func algo(name string) (otp.Algorithm, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "", "SHA1":
		return otp.AlgorithmSHA1, nil
	case "SHA256":
		return otp.AlgorithmSHA256, nil
	case "SHA512":
		return otp.AlgorithmSHA512, nil
	default:
		return 0, fmt.Errorf("unsupported algorithm %q", name)
	}
}

// digits maps an int to pquerna's Digits enum.
func digits(n int) (otp.Digits, error) {
	switch n {
	case 0, 6:
		return otp.DigitsSix, nil
	case 8:
		return otp.DigitsEight, nil
	default:
		return 0, fmt.Errorf("unsupported digit count %d", n)
	}
}

// Generate produces the current code for the given params.
func Generate(p Params) (Code, error) {
	a, err := algo(p.Algorithm)
	if err != nil {
		return Code{}, err
	}
	d, err := digits(p.Digits)
	if err != nil {
		return Code{}, err
	}
	period := p.Period
	if period <= 0 {
		period = 30
	}

	switch strings.ToLower(p.Type) {
	case "hotp":
		code, err := hotp.GenerateCodeCustom(p.Secret, uint64(p.Counter), hotp.ValidateOpts{
			Digits:    d,
			Algorithm: a,
		})
		if err != nil {
			return Code{}, err
		}
		return Code{Code: code, Remaining: 0, Period: period}, nil
	default:
		code, err := totp.GenerateCodeCustom(p.Secret, time.Now(), totp.ValidateOpts{
			Period:    uint(period),
			Skew:      0,
			Digits:    d,
			Algorithm: a,
		})
		if err != nil {
			return Code{}, err
		}
		remaining := period - int(time.Now().Unix()%int64(period))
		return Code{Code: code, Remaining: remaining, Period: period}, nil
	}
}

// GenerateFor generates a code for a stored entry.
func GenerateFor(e store.OTPEntry) (Code, error) {
	return Generate(Params{
		Secret:    e.Secret,
		Algorithm: e.Algorithm,
		Digits:    e.Digits,
		Period:    e.Period,
		Counter:   e.Counter,
		Type:      e.Type,
	})
}

// ParseURI parses an otpauth:// URI into an OTPEntry (without ID/timestamps).
func ParseURI(raw string) (store.OTPEntry, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return store.OTPEntry{}, fmt.Errorf("parse otpauth uri: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "otpauth") {
		return store.OTPEntry{}, fmt.Errorf("not an otpauth uri (scheme=%q)", u.Scheme)
	}

	typ := strings.ToLower(u.Host) // totp / hotp
	if typ != "totp" && typ != "hotp" {
		return store.OTPEntry{}, fmt.Errorf("unsupported otp type %q", typ)
	}

	label := strings.TrimPrefix(u.Path, "/")
	issuer := u.Query().Get("issuer")
	account := label
	if i := strings.Index(label, ":"); i >= 0 {
		if issuer == "" {
			issuer = strings.TrimSpace(label[:i])
		}
		account = strings.TrimSpace(label[i+1:])
	}

	e := store.OTPEntry{
		Title:     account,
		Issuer:    issuer,
		Account:   account,
		Secret:    strings.ToUpper(strings.TrimSpace(u.Query().Get("secret"))),
		Algorithm: valueOr(u.Query().Get("algorithm"), "SHA1"),
		Digits:    atoiOr(u.Query().Get("digits"), 6),
		Period:    atoiOr(u.Query().Get("period"), 30),
		Type:      typ,
	}
	if e.Secret == "" {
		return store.OTPEntry{}, fmt.Errorf("otpauth uri missing secret")
	}
	return e, nil
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.ToUpper(strings.TrimSpace(v))
}

func atoiOr(v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	var n int
	for _, r := range v {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return def
	}
	return n
}

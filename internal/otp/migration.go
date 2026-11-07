package otp

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
)

// Google Authenticator's "Export accounts" feature shows QR codes containing
// otpauth-migration://offline?data=<base64url protobuf> URIs. Each QR carries
// one batch of entries (up to 10). The payload is a MigrationPayload protobuf:
//
//	message MigrationPayload {
//	  repeated OtpParameters otp_parameters = 1;
//	  int32 version = 2; int32 batch_size = 3;
//	  int32 batch_index = 4; int32 batch_id = 5;
//	}
//	message OtpParameters {
//	  bytes secret = 1; string name = 2; string issuer = 3;
//	  Algorithm algorithm = 4; DigitCount digits = 5;
//	  OtpType type = 6; int64 counter = 7;
//	}
//
// The wire format is simple and fixed, so we decode it by hand instead of
// pulling in a protobuf runtime.

// MigrationResult carries the decoded batch.
type MigrationResult struct {
	Entries    []store.OTPEntry
	Version    int
	BatchSize  int
	BatchIndex int
	BatchID    int
}

// ParseMigrationURI decodes an otpauth-migration:// URI into OTP entries.
func ParseMigrationURI(raw string) (*MigrationResult, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parse migration uri: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "otpauth-migration") {
		return nil, fmt.Errorf("not an otpauth-migration uri (scheme=%q)", u.Scheme)
	}
	data := u.Query().Get("data")
	if data == "" {
		return nil, fmt.Errorf("migration uri missing data parameter")
	}
	payload, err := decodeB64(data)
	if err != nil {
		return nil, fmt.Errorf("decode migration data: %w", err)
	}
	res, err := parseMigrationPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("parse migration payload: %w", err)
	}
	if len(res.Entries) == 0 {
		return nil, fmt.Errorf("migration payload contains no entries")
	}
	return res, nil
}

// decodeB64 tries the base64 variants Google may emit (url no-pad, url pad, std).
func decodeB64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

func parseMigrationPayload(b []byte) (*MigrationResult, error) {
	res := &MigrationResult{}
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, fmt.Errorf("bad field key")
		}
		b = b[n:]
		field, wt := int(key>>3), int(key&7)
		switch wt {
		case 0: // varint
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, fmt.Errorf("bad varint for field %d", field)
			}
			b = b[n:]
			switch field {
			case 2:
				res.Version = int(v)
			case 3:
				res.BatchSize = int(v)
			case 4:
				res.BatchIndex = int(v)
			case 5:
				res.BatchID = int(v)
			}
		case 2: // length-delimited
			l, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, fmt.Errorf("bad length for field %d", field)
			}
			b = b[n:]
			if uint64(len(b)) < l {
				return nil, fmt.Errorf("truncated field %d", field)
			}
			chunk := b[:l]
			b = b[l:]
			if field == 1 {
				e, err := parseOtpParameters(chunk)
				if err != nil {
					return nil, err
				}
				res.Entries = append(res.Entries, e)
			}
		default:
			return nil, fmt.Errorf("unsupported wire type %d (field %d)", wt, field)
		}
	}
	return res, nil
}

func parseOtpParameters(b []byte) (store.OTPEntry, error) {
	e := store.OTPEntry{
		Type:      "totp",
		Algorithm: "SHA1",
		Digits:    6,
		Period:    30,
	}
	var secret []byte
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return e, fmt.Errorf("bad field key in otp_parameters")
		}
		b = b[n:]
		field, wt := int(key>>3), int(key&7)
		switch wt {
		case 0:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return e, fmt.Errorf("bad varint in otp_parameters field %d", field)
			}
			b = b[n:]
			switch field {
			case 4: // algorithm enum
				switch v {
				case 2:
					e.Algorithm = "SHA256"
				case 3:
					e.Algorithm = "SHA512"
				default:
					e.Algorithm = "SHA1"
				}
			case 5: // digit count enum
				if v == 2 {
					e.Digits = 8
				} else {
					e.Digits = 6
				}
			case 6: // otp type enum
				if v == 1 {
					e.Type = "hotp"
				} else {
					e.Type = "totp"
				}
			case 7:
				e.Counter = int64(v)
			}
		case 2:
			l, n := binary.Uvarint(b)
			if n <= 0 {
				return e, fmt.Errorf("bad length in otp_parameters field %d", field)
			}
			b = b[n:]
			if uint64(len(b)) < l {
				return e, fmt.Errorf("truncated otp_parameters field %d", field)
			}
			chunk := b[:l]
			b = b[l:]
			switch field {
			case 1:
				secret = chunk
			case 2:
				e.Account = string(chunk)
			case 3:
				if s := string(chunk); s != "" {
					e.Issuer = s
				}
			}
		default:
			return e, fmt.Errorf("unsupported wire type %d in otp_parameters", wt)
		}
	}
	if len(secret) == 0 {
		return e, fmt.Errorf("entry %q missing secret", e.Account)
	}
	// Google packs "Issuer:Account" into the name field even when the
	// dedicated issuer field is set, so always split on the first colon.
	if i := strings.Index(e.Account, ":"); i >= 0 {
		nameIssuer := strings.TrimSpace(e.Account[:i])
		e.Account = strings.TrimSpace(e.Account[i+1:])
		if e.Issuer == "" {
			e.Issuer = nameIssuer
		}
	}
	e.Title = e.Account
	e.Secret = base32.StdEncoding.EncodeToString(secret)
	return e, nil
}

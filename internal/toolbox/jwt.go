package toolbox

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JWTResult holds the decoded parts of a JSON Web Token. The signature is
// surfaced but not verified — this is a debug helper, not an auth check.
type JWTResult struct {
	Valid       bool           `json:"valid"`
	Header      map[string]any `json:"header"`
	Payload     map[string]any `json:"payload"`
	HeaderJSON  string         `json:"headerJson"`
	PayloadJSON string         `json:"payloadJson"`
	Signature   string         `json:"signature"`
	Alg         string         `json:"alg"`
	IssuedAt    int64          `json:"issuedAt,omitempty"`
	NotBefore   int64          `json:"notBefore,omitempty"`
	ExpiresAt   int64          `json:"expiresAt,omitempty"`
	Expired     bool           `json:"expired"`
	ExpiresIn   string         `json:"expiresIn,omitempty"`
}

// DecodeJWT splits and decodes a JWT's header and payload. It accepts a
// leading "Bearer " prefix and padding-less base64 segments.
func DecodeJWT(token string) (JWTResult, error) {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return JWTResult{}, fmt.Errorf("jwt_invalid")
	}

	decode := func(seg string) ([]byte, error) {
		seg = strings.TrimRight(strings.TrimSpace(seg), "=")
		return base64.RawURLEncoding.DecodeString(seg)
	}

	hb, err := decode(parts[0])
	if err != nil {
		return JWTResult{}, fmt.Errorf("jwt_header_decode: %s", err.Error())
	}
	pb, err := decode(parts[1])
	if err != nil {
		return JWTResult{}, fmt.Errorf("jwt_payload_decode: %s", err.Error())
	}

	var header, payload map[string]any
	if err := json.Unmarshal(hb, &header); err != nil {
		return JWTResult{}, fmt.Errorf("jwt_header_json: %s", err.Error())
	}
	if err := json.Unmarshal(pb, &payload); err != nil {
		return JWTResult{}, fmt.Errorf("jwt_payload_json: %s", err.Error())
	}

	pretty := func(b []byte) string {
		var v any
		if json.Unmarshal(b, &v) != nil {
			return string(b)
		}
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return string(b)
		}
		return string(out)
	}

	res := JWTResult{
		Valid:       true,
		Header:      header,
		Payload:     payload,
		HeaderJSON:  pretty(hb),
		PayloadJSON: pretty(pb),
	}
	if len(parts) > 2 {
		res.Signature = parts[2]
	}
	if alg, ok := header["alg"].(string); ok {
		res.Alg = alg
	}

	now := time.Now().Unix()
	if v, ok := numClaim(payload["iat"]); ok {
		res.IssuedAt = v
	}
	if v, ok := numClaim(payload["nbf"]); ok {
		res.NotBefore = v
	}
	if v, ok := numClaim(payload["exp"]); ok {
		res.ExpiresAt = v
		res.Expired = v < now
		if !res.Expired {
			res.ExpiresIn = (time.Duration(v-now) * time.Second).String()
		}
	}
	return res, nil
}

// numClaim coerces a JWT numeric claim (JSON numbers decode to float64) into
// an int64 epoch second.
func numClaim(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int64:
		return n, true
	}
	return 0, false
}

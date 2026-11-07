package toolbox

import (
	"crypto/rand"
	"fmt"
	"time"
)

// uuidV4 generates an RFC 4122 version-4 UUID string.
func uuidV4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID generates a lexicographically sortable ULID (Crockford base32, 48-bit
// timestamp + 80 bits of randomness).
func ULID() (string, error) {
	ts := uint64(time.Now().UnixMilli())
	buf := make([]byte, 26)
	for i := 9; i >= 0; i-- {
		buf[i] = ulidAlphabet[ts&31]
		ts >>= 5
	}
	rnd := make([]byte, 10)
	if _, err := rand.Read(rnd); err != nil {
		return "", err
	}
	// Encode 80 random bits into the last 16 chars using 5 bits each.
	for i := 0; i < 16; i++ {
		bitPos := i * 5
		var v byte
		for j := 0; j < 5; j++ {
			byteIdx := (bitPos + j) / 8
			bitIdx := 7 - (bitPos+j)%8
			bit := (rnd[byteIdx] >> uint(bitIdx)) & 1
			v = (v << 1) | bit
		}
		buf[10+i] = ulidAlphabet[v]
	}
	return string(buf), nil
}

// ValidateULID reports whether s is a well-formed 26-char ULID.
func ValidateULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	for _, r := range s {
		found := false
		for _, c := range ulidAlphabet {
			if r == c {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// UUIDs generates n random UUID v4 strings.
func UUIDs(n int) ([]string, error) {
	if n <= 0 || n > 1000 {
		n = 1
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		u, err := uuidV4()
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// ULIDs generates n random ULID strings.
func ULIDs(n int) ([]string, error) {
	if n <= 0 || n > 1000 {
		n = 1
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		u, err := ULID()
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

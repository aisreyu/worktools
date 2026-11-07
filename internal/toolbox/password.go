package toolbox

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// ---- password generation ----

// PasswordOptions controls random password generation. Level is "low",
// "medium" or "high"; the Use* flags let the user override the level's
// default character classes. CustomSymbols, when set, replaces the default
// symbol set.
type PasswordOptions struct {
	Level         string `json:"level"`         // low | medium | high
	Length        int    `json:"length"`        // desired length
	UseLower      *bool  `json:"useLower"`      // override: include a-z
	UseUpper      *bool  `json:"useUpper"`      // override: include A-Z
	UseDigits     *bool  `json:"useDigits"`     // override: include 0-9
	UseSymbols    *bool  `json:"useSymbols"`    // override: include symbols
	CustomSymbols string `json:"customSymbols"` // replace default symbol set
	NoAmbiguous   bool   `json:"noAmbiguous"`   // exclude 0O1lI|`'
}

const (
	lowerSet   = "abcdefghijklmnopqrstuvwxyz"
	upperSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitSet   = "0123456789"
	symbolSet  = "!@#$%^&*()-_=+[]{};:,.<>?/~"
	ambiguous  = "0O1lI|`'"
	emailDom   = "gmail.com,outlook.com,qq.com,163.com,126.com,hotmail.com,yahoo.com,foxmail.com"
	chinaPhone = "13,14,15,16,17,18,19"
)

// GeneratePassword produces a random password honouring the requested
// complexity. At least one character from every enabled class is guaranteed.
func GeneratePassword(o PasswordOptions) (string, error) {
	if o.Length < 4 {
		o.Length = 12
	}
	if o.Length > 128 {
		o.Length = 128
	}
	useLower, useUpper, useDigits, useSymbols := levelDefaults(o.Level)
	if o.UseLower != nil {
		useLower = *o.UseLower
	}
	if o.UseUpper != nil {
		useUpper = *o.UseUpper
	}
	if o.UseDigits != nil {
		useDigits = *o.UseDigits
	}
	if o.UseSymbols != nil {
		useSymbols = *o.UseSymbols
	}

	symbols := o.CustomSymbols
	if strings.TrimSpace(symbols) == "" {
		symbols = symbolSet
	}

	var classes []string
	if useLower {
		classes = append(classes, lowerSet)
	}
	if useUpper {
		classes = append(classes, upperSet)
	}
	if useDigits {
		classes = append(classes, digitSet)
	}
	if useSymbols {
		classes = append(classes, symbols)
	}
	if len(classes) == 0 {
		return "", fmt.Errorf("password_need_charset")
	}
	if o.NoAmbiguous {
		for i := range classes {
			classes[i] = stripAmbiguous(classes[i])
		}
	}

	// Guarantee one char per class first, then fill the rest from the union.
	var pw []byte
	for _, c := range classes {
		ch, err := randFrom(c)
		if err != nil {
			return "", err
		}
		pw = append(pw, ch)
	}
	union := strings.Join(classes, "")
	for len(pw) < o.Length {
		ch, err := randFrom(union)
		if err != nil {
			return "", err
		}
		pw = append(pw, ch)
	}
	// Fisher-Yates shuffle so the guaranteed class chars aren't predictable.
	for i := len(pw) - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return "", err
		}
		pw[i], pw[j] = pw[j], pw[i]
	}
	return string(pw), nil
}

func levelDefaults(level string) (lower, upper, digits, symbols bool) {
	switch strings.ToLower(level) {
	case "low":
		return true, false, true, false
	case "high":
		return true, true, true, true
	default: // medium
		return true, true, true, false
	}
}

func stripAmbiguous(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !strings.ContainsRune(ambiguous, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func randFrom(charset string) (byte, error) {
	if charset == "" {
		return 0, fmt.Errorf("empty charset")
	}
	n, err := randInt(len(charset))
	if err != nil {
		return 0, err
	}
	return charset[n], nil
}

func randInt(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("randInt: n must be positive")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// ---- prefix/suffix join ----

// JoinWithAffixes wraps every non-empty line with prefix and suffix (e.g.
// "http://" + line + ".net") and joins the results with sep.
func JoinWithAffixes(input, prefix, suffix, sep string) string {
	if sep == "" {
		sep = "\n"
	}
	items := SplitLines(input)
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = prefix + it + suffix
	}
	return strings.Join(out, sep)
}

// ---- random identity data ----

// RandomEmails generates n random email addresses using common providers.
func RandomEmails(n int) ([]string, error) {
	doms := strings.Split(emailDom, ",")
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		local, err := randomLocalPart()
		if err != nil {
			return nil, err
		}
		di, err := randInt(len(doms))
		if err != nil {
			return nil, err
		}
		out = append(out, local+"@"+doms[di])
	}
	return out, nil
}

func randomLocalPart() (string, error) {
	patterns := []string{
		"word+digits",
		"word.word+digits",
		"word+digits+word",
		"letters",
	}
	words := []string{
		"alice", "bob", "carol", "dave", "emma", "frank", "grace", "henry",
		"iris", "jack", "kate", "liam", "mia", "noah", "olivia", "peter",
		"quinn", "rose", "sam", "tina", "victor", "wendy", "xavier", "yuki", "zoe",
		"chen", "wang", "li", "zhang", "liu", "yang", "huang", "zhao", "wu", "zhou",
	}
	p, err := randInt(len(patterns))
	if err != nil {
		return "", err
	}
	w1i, err := randInt(len(words))
	if err != nil {
		return "", err
	}
	w1 := words[w1i]
	switch patterns[p] {
	case "word.word+digits":
		w2i, err := randInt(len(words))
		if err != nil {
			return "", err
		}
		num, err := randInt(1000)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s%d", w1, words[w2i], num), nil
	case "word+digits+word":
		w2i, err := randInt(len(words))
		if err != nil {
			return "", err
		}
		num, err := randInt(100)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%d%s", w1, num, words[w2i]), nil
	case "letters":
		extra, err := randInt(6)
		if err != nil {
			return "", err
		}
		n := 6 + extra
		var b strings.Builder
		for i := 0; i < n; i++ {
			ch, err := randFrom(lowerSet)
			if err != nil {
				return "", err
			}
			b.WriteByte(ch)
		}
		return b.String(), nil
	default: // word+digits
		num, err := randInt(10000)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%d", w1, num), nil
	}
}

// RandomPhones generates n random Chinese mobile numbers (13x-19x segments).
func RandomPhones(n int) ([]string, error) {
	segs := strings.Split(chinaPhone, ",")
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		si, err := randInt(len(segs))
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		b.WriteString(segs[si])
		for j := 0; j < 9; j++ {
			d, err := randFrom(digitSet)
			if err != nil {
				return nil, err
			}
			b.WriteByte(d)
		}
		out = append(out, b.String())
	}
	return out, nil
}

// areaCode/sequence data for ID generation: representative codes covering
// several provinces; birth dates are random within 1960-2005.
var idAreaCodes = []string{
	"110101", "310101", "440305", "330102", "510107", // 京沪深杭蓉
	"420102", "610103", "500103", "120101", "370202", // 武汉西安渝津青
}

// RandomIDCards generates n random Chinese resident ID numbers (18 digits
// with a valid checksum). Data is entirely synthetic — for testing only.
func RandomIDCards(n int) ([]string, error) {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ai, err := randInt(len(idAreaCodes))
		if err != nil {
			return nil, err
		}
		yearExtra, err := randInt(46)
		if err != nil {
			return nil, err
		}
		monthExtra, err := randInt(12)
		if err != nil {
			return nil, err
		}
		dayExtra, err := randInt(28)
		if err != nil {
			return nil, err
		}
		year := 1960 + yearExtra
		month := 1 + monthExtra
		day := 1 + dayExtra
		var b strings.Builder
		b.WriteString(idAreaCodes[ai])
		fmt.Fprintf(&b, "%04d%02d%02d", year, month, day)
		for j := 0; j < 3; j++ {
			d, err := randFrom(digitSet)
			if err != nil {
				return nil, err
			}
			b.WriteByte(d)
		}
		body := b.String() // 17 digits
		out = append(out, body+idChecksum(body))
	}
	return out, nil
}

// idChecksum computes the GB11643 mod-11 check character.
func idChecksum(body17 string) string {
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	marks := "10X98765432"
	sum := 0
	for i, r := range body17 {
		d := int(r - '0')
		sum += d * weights[i]
	}
	return string(marks[sum%11])
}

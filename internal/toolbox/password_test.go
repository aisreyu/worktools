package toolbox

import (
	"regexp"
	"strings"
	"testing"
)

func TestGeneratePasswordLevels(t *testing.T) {
	cases := []struct {
		level                  string
		wantLower, wantUpper   bool
		wantDigits, wantSyms   bool
	}{
		{"low", true, false, true, false},
		{"medium", true, true, true, false},
		{"high", true, true, true, true},
	}
	for _, c := range cases {
		pw, err := GeneratePassword(PasswordOptions{Level: c.level, Length: 16})
		if err != nil {
			t.Fatalf("level %s: %v", c.level, err)
		}
		if len(pw) != 16 {
			t.Errorf("level %s: len=%d", c.level, len(pw))
		}
		check := func(name string, set string, want bool) {
			has := strings.ContainsAny(pw, set)
			if has != want {
				t.Errorf("level %s: %s has=%v want=%v (pw=%q)", c.level, name, has, want, pw)
			}
		}
		check("lower", lowerSet, c.wantLower)
		check("upper", upperSet, c.wantUpper)
		check("digits", digitSet, c.wantDigits)
		check("symbols", symbolSet, c.wantSyms)
	}
}

func TestGeneratePasswordOverrides(t *testing.T) {
	// digits only
	no := false
	pw, err := GeneratePassword(PasswordOptions{Level: "high", Length: 12, UseLower: &no, UseUpper: &no, UseSymbols: &no})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(pw, lowerSet+upperSet+symbolSet) {
		t.Errorf("digits-only pw has other chars: %q", pw)
	}
	// custom symbols
	yes := true
	pw, err = GeneratePassword(PasswordOptions{Level: "high", Length: 20, CustomSymbols: "@#$", UseSymbols: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(pw, symbolSet) && !strings.ContainsAny(pw, "@#$") {
		t.Errorf("custom symbols ignored: %q", pw)
	}
	// no ambiguous
	pw, err = GeneratePassword(PasswordOptions{Level: "high", Length: 32, NoAmbiguous: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(pw, ambiguous) {
		t.Errorf("ambiguous chars present: %q", pw)
	}
	// all classes disabled → error
	if _, err := GeneratePassword(PasswordOptions{Level: "low", UseLower: &no, UseDigits: &no}); err == nil {
		t.Error("expected error when all classes disabled")
	}
}

func TestJoinWithAffixes(t *testing.T) {
	in := "api.apitd\nai.apitd\npi.apitd"
	got := JoinWithAffixes(in, "http://", ".net", "\n")
	want := "http://api.apitd.net\nhttp://ai.apitd.net\nhttp://pi.apitd.net"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// comma join
	got = JoinWithAffixes("a\nb", "[", "]", ",")
	if got != "[a],[b]" {
		t.Errorf("comma: %q", got)
	}
}

func TestRandomEmails(t *testing.T) {
	re := regexp.MustCompile(`^[a-z0-9.]+@(gmail|outlook|qq|163|126|hotmail|yahoo|foxmail)\.com$`)
	emails, err := RandomEmails(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 20 {
		t.Fatalf("want 20, got %d", len(emails))
	}
	seen := map[string]bool{}
	for _, e := range emails {
		if !re.MatchString(e) {
			t.Errorf("bad email: %q", e)
		}
		seen[e] = true
	}
	if len(seen) < 5 {
		t.Logf("only %d unique of 20 (informational)", len(seen))
	}
}

func TestRandomPhones(t *testing.T) {
	re := regexp.MustCompile(`^1[3-9]\d{9}$`)
	phones, err := RandomPhones(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range phones {
		if !re.MatchString(p) {
			t.Errorf("bad phone: %q", p)
		}
	}
}

func TestRandomIDCards(t *testing.T) {
	re := regexp.MustCompile(`^\d{17}[\dX]$`)
	ids, err := RandomIDCards(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !re.MatchString(id) {
			t.Errorf("bad id format: %q", id)
			continue
		}
		if idChecksum(id[:17]) != string(id[17]) {
			t.Errorf("checksum mismatch: %q", id)
		}
	}
}

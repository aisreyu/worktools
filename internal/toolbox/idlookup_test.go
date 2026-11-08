package toolbox

import "testing"

func TestLookupPhone(t *testing.T) {
	cases := []struct {
		num      string
		ok       bool
		operator string
		province string
	}{
		{"13800138000", true, "中国移动", "北京"},
		{"13100000000", true, "中国联通", "山西"},
		{"13300000000", true, "中国电信", "广西"},
		{"19200000000", true, "中国广电", ""},
		{"12345", false, "", ""},
		{"23800138000", false, "", ""},
	}
	for _, c := range cases {
		got := LookupPhone(c.num)
		if got.OK != c.ok {
			t.Fatalf("%s: ok=%v want %v (%s)", c.num, got.OK, c.ok, got.Message)
		}
		if !c.ok {
			continue
		}
		if got.Operator != c.operator {
			t.Errorf("%s: operator=%q want %q", c.num, got.Operator, c.operator)
		}
		if c.province != "" && got.Province != c.province {
			t.Errorf("%s: province=%q want %q", c.num, got.Province, c.province)
		}
	}
}

func TestLookupPhoneSpaceTolerant(t *testing.T) {
	got := LookupPhone(" 138 0013 8000 ")
	if !got.OK {
		t.Fatalf("expected ok, got %s", got.Message)
	}
}

func TestLookupIDCardValid(t *testing.T) {
	got := LookupIDCard("110101199001010015")
	if !got.Valid {
		t.Fatalf("expected valid, reasons=%v", got.Reasons)
	}
	if !got.ChecksumOK {
		t.Error("checksum should pass")
	}
	if got.Province != "北京市" || got.District != "东城区" {
		t.Errorf("region=%s/%s/%s", got.Province, got.City, got.District)
	}
	if got.Birthday != "1990-01-01" || got.Gender != "男" {
		t.Errorf("birthday=%s gender=%s", got.Birthday, got.Gender)
	}
}

func TestLookupIDCardBadChecksum(t *testing.T) {
	got := LookupIDCard("110101199001010010")
	if got.Valid {
		t.Fatal("expected invalid")
	}
	if got.ChecksumOK {
		t.Error("checksum should fail")
	}
}

func TestLookupIDCardBadRegion(t *testing.T) {
	got := LookupIDCard("99999919900101001X")
	if got.Valid {
		t.Fatal("expected invalid region")
	}
}

func TestLookupIDCardBadBirthday(t *testing.T) {
	got := LookupIDCard("110101199013010011")
	if got.Valid {
		t.Fatal("expected invalid birthday")
	}
}

func TestLookupIDCard15(t *testing.T) {
	got := LookupIDCard("110101900101001")
	if got.Birthday != "1990-01-01" {
		t.Errorf("birthday=%s", got.Birthday)
	}
	if got.Province != "北京市" {
		t.Errorf("province=%s", got.Province)
	}
}

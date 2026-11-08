package toolbox

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// PhoneResult is the offline attribution result for a mobile number.
type PhoneResult struct {
	Input       string `json:"input"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	Province    string `json:"province"`
	City        string `json:"city"`
	Operator    string `json:"operator"`
	AreaCode    string `json:"areaCode"`
	ZipCode     string `json:"zipCode"`
	DataVersion string `json:"dataVersion"`
}

// IDCardResult is the offline lookup + validity result for an ID card number.
type IDCardResult struct {
	Input      string   `json:"input"`
	Valid      bool     `json:"valid"`
	Reasons    []string `json:"reasons"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	District   string   `json:"district"`
	Birthday   string   `json:"birthday"`
	Gender     string   `json:"gender"`
	Length     int      `json:"length"`
	ChecksumOK bool     `json:"checksumOk"`
}

var phoneOperators = map[int]string{
	1: "中国移动",
	2: "中国联通",
	3: "中国电信",
	4: "电信虚拟运营商",
	5: "联通虚拟运营商",
	6: "移动虚拟运营商",
	7: "中国广电",
	8: "广电虚拟运营商",
}

// LookupPhone resolves the operator and region of a Chinese mobile number from
// the bundled offline snapshot database.
func LookupPhone(number string) PhoneResult {
	num := strings.TrimSpace(number)
	num = strings.NewReplacer(" ", "", "-", "", "+86", "").Replace(num)
	res := PhoneResult{Input: strings.TrimSpace(number), DataVersion: phoneSnapshotVersion}
	if len(num) != 11 || !allDigits(num) || num[0] != '1' {
		res.Message = "请输入 11 位中国大陆手机号"
		return res
	}
	pref := parseInt(num[:7])
	db := loadPhones()
	idx := sort.Search(len(db.recs), func(i int) bool { return db.recs[i].prefix >= pref })
	// Longest-prefix match is unnecessary here: prefixes are stored at 7-digit
	// granularity, so an exact match on the first 7 digits is sufficient.
	if idx >= len(db.recs) || db.recs[idx].prefix != pref {
		res.Message = "未收录该号段的归属地信息"
		return res
	}
	rec := db.recs[idx]
	res.OK = true
	res.Operator = phoneOperators[rec.opType]
	if res.Operator == "" {
		res.Operator = "未知"
	}
	if rec.strIdx >= 0 && rec.strIdx < len(db.strs) {
		parts := strings.Split(db.strs[rec.strIdx], "|")
		if len(parts) > 0 {
			res.Province = parts[0]
		}
		if len(parts) > 1 {
			res.City = parts[1]
		}
		if len(parts) > 2 {
			res.ZipCode = parts[2]
		}
		if len(parts) > 3 {
			res.AreaCode = parts[3]
		}
	}
	return res
}

// LookupIDCard resolves the region of an ID card number and performs a local
// validity check (length, character set, region code, birth date, checksum).
//
// It cannot verify the number against any authoritative 公安 registry; the
// result reflects structural/checksum validity only.
func LookupIDCard(number string) IDCardResult {
	in := strings.TrimSpace(number)
	norm := strings.ToUpper(strings.ReplaceAll(in, " ", ""))
	res := IDCardResult{Input: in, Length: len([]rune(norm))}
	reg := loadRegions()

	switch len(norm) {
	case 18:
		checkIDCard18(norm, &res, reg)
	case 15:
		checkIDCard15(norm, &res, reg)
	default:
		res.Reasons = append(res.Reasons, "长度不是 15 位或 18 位")
	}
	res.Valid = len(res.Reasons) == 0
	return res
}

func checkIDCard18(s string, res *IDCardResult, reg *regionDB) {
	if !allDigits(s[:17]) || !(s[17] >= '0' && s[17] <= '9' || s[17] == 'X') {
		res.Reasons = append(res.Reasons, "含非法字符")
		return
	}
	fillRegion(s[:6], res, reg)
	fillBirthday(s[6:14], res)
	res.Gender = genderFrom(s[16])
	ok := idChecksum(s[:17]) == string(s[17])
	res.ChecksumOK = ok
	if !ok {
		res.Reasons = append(res.Reasons, "校验码不正确")
	}
}

func checkIDCard15(s string, res *IDCardResult, reg *regionDB) {
	if !allDigits(s) {
		res.Reasons = append(res.Reasons, "含非法字符")
		return
	}
	fillRegion(s[:6], res, reg)
	// 15-digit cards encode the year with 2 digits; assume 19xx.
	fillBirthday("19"+s[6:12], res)
	res.Gender = genderFrom(s[14])
	res.Reasons = append(res.Reasons, "15 位号码为旧版格式，已按规则推算出生日期（无校验码）")
}

func fillRegion(code string, res *IDCardResult, reg *regionDB) {
	c := parseInt(code)
	if name, ok := reg.provinces[c/10000]; ok {
		res.Province = name
	}
	if name, ok := reg.cities[c/100]; ok {
		res.City = name
	}
	if name, ok := reg.areas[c]; ok {
		res.District = name
	}
	if res.Province == "" && res.City == "" && res.District == "" {
		res.Reasons = append(res.Reasons, "行政区划代码不存在（"+code+"）")
	}
}

func fillBirthday(ymd string, res *IDCardResult) {
	if len(ymd) != 8 {
		res.Reasons = append(res.Reasons, "出生日期格式错误")
		return
	}
	t, err := time.Parse("20060102", ymd)
	if err != nil {
		res.Reasons = append(res.Reasons, "出生日期无效（"+ymd+"）")
		return
	}
	if t.After(time.Now()) || t.Year() < 1900 {
		res.Reasons = append(res.Reasons, "出生日期超出合理范围")
		return
	}
	res.Birthday = t.Format("2006-01-02")
}

func genderFrom(d byte) string {
	if (d-'0')%2 == 1 {
		return "男"
	}
	return "女"
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

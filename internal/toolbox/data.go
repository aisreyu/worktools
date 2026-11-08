package toolbox

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"sync"
)

// Bundled offline lookup data. No network access is performed at runtime.
//
//	provinces.bin.gz  code(2) -> name
//	cities.bin.gz     code(4), provinceCode(2) -> name
//	areas.bin.gz      code(6), cityCode(4), provinceCode(2) -> name
//	phone.bin.gz      mobile prefix -> operator + region (snapshot)
//
//go:embed data/provinces.bin.gz data/cities.bin.gz data/areas.bin.gz data/phone.bin.gz
var dataFS embed.FS

// phoneSnapshotVersion identifies the bundled mobile-attribution dataset.
const phoneSnapshotVersion = "2026.04"

func gunzip(b []byte) []byte {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil
	}
	return out
}

// ---- varint reader ----

type vreader struct {
	b []byte
	i int
}

func (r *vreader) uvarint() uint64 {
	var x uint64
	var s uint
	for r.i < len(r.b) {
		c := r.b[r.i]
		r.i++
		if c < 0x80 {
			x |= uint64(c) << s
			return x
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return x
}

func (r *vreader) str() string {
	n := int(r.uvarint())
	if r.i+n > len(r.b) {
		n = len(r.b) - r.i
	}
	s := string(r.b[r.i : r.i+n])
	r.i += n
	return s
}

// ---- region DB ----

type regionDB struct {
	provinces map[int]string
	cities    map[int]string // code -> name
	cityProv  map[int]int    // city code -> province code
	areas     map[int]string // code -> name
}

var (
	regionsOnce sync.Once
	regions     *regionDB
)

func loadRegions() *regionDB {
	regionsOnce.Do(func() {
		regions = &regionDB{
			provinces: map[int]string{},
			cities:    map[int]string{},
			cityProv:  map[int]int{},
			areas:     map[int]string{},
		}
		if b, err := dataFS.ReadFile("data/provinces.bin.gz"); err == nil {
			r := &vreader{b: gunzip(b)}
			n := int(r.uvarint())
			for i := 0; i < n; i++ {
				code := int(r.uvarint())
				regions.provinces[code] = r.str()
			}
		}
		if b, err := dataFS.ReadFile("data/cities.bin.gz"); err == nil {
			r := &vreader{b: gunzip(b)}
			n := int(r.uvarint())
			for i := 0; i < n; i++ {
				code := int(r.uvarint())
				prov := int(r.uvarint())
				regions.cities[code] = r.str()
				regions.cityProv[code] = prov
			}
		}
		if b, err := dataFS.ReadFile("data/areas.bin.gz"); err == nil {
			r := &vreader{b: gunzip(b)}
			n := int(r.uvarint())
			for i := 0; i < n; i++ {
				code := int(r.uvarint())
				r.uvarint() // city code (unused: derivable from code)
				r.uvarint() // province code (unused)
				regions.areas[code] = r.str()
			}
		}
	})
	return regions
}

// ---- phone DB ----

type phoneRec struct {
	prefix int
	opType int
	strIdx int
}

type phoneDB struct {
	recs    []phoneRec
	strs    []string
	minPref int
}

var (
	phoneOnce sync.Once
	phones    *phoneDB
)

func loadPhones() *phoneDB {
	phoneOnce.Do(func() {
		b, err := dataFS.ReadFile("data/phone.bin.gz")
		if err != nil {
			phones = &phoneDB{}
			return
		}
		r := &vreader{b: gunzip(b)}
		ns := int(r.uvarint())
		phone := &phoneDB{strs: make([]string, ns)}
		for i := 0; i < ns; i++ {
			phone.strs[i] = r.str()
		}
		nc := int(r.uvarint())
		phone.recs = make([]phoneRec, 0, nc)
		prev := 0
		for i := 0; i < nc; i++ {
			prev += int(r.uvarint())
			t := int(r.b[r.i])
			r.i++
			si := int(r.uvarint())
			phone.recs = append(phone.recs, phoneRec{prefix: prev, opType: t, strIdx: si})
		}
		if len(phone.recs) > 0 {
			phone.minPref = phone.recs[0].prefix
		}
		phones = phone
	})
	return phones
}

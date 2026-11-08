package toolbox

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"path/filepath"
	"testing"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

func TestConvertToEachFormat(t *testing.T) {
	p := writeTestJPEG(t, 640, 480)
	cases := []struct {
		target string
		mime   string
		ext    string
	}{
		{"jpeg", "image/jpeg", ".jpg"},
		{"png", "image/png", ".png"},
		{"gif", "image/gif", ".gif"},
		{"bmp", "image/bmp", ".bmp"},
		{"tiff", "image/tiff", ".tiff"},
	}
	for _, c := range cases {
		res, data, err := ConvertFile(p, ConvertOptions{Target: c.target})
		if err != nil {
			t.Fatalf("convert %s: %v", c.target, err)
		}
		if res.Mime != c.mime {
			t.Fatalf("%s mime: got %s want %s", c.target, res.Mime, c.mime)
		}
		if res.ToFormat != c.target {
			t.Fatalf("%s format: got %s", c.target, res.ToFormat)
		}
		if len(data) == 0 || res.Size != len(data) {
			t.Fatalf("%s size mismatch", c.target)
		}
		if res.Width != 640 || res.Height != 480 {
			t.Fatalf("%s dims: %dx%d", c.target, res.Width, res.Height)
		}
		out := ConvertOutputPath(p, "", c.target, "_conv")
		if filepath.Ext(out) != c.ext {
			t.Fatalf("%s out ext: %s", c.target, filepath.Ext(out))
		}
	}
}

func TestConvertDecodesBack(t *testing.T) {
	p := writeTestJPEG(t, 320, 240)
	_, data, err := ConvertFile(p, ConvertOptions{Target: "png"})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png result: %v", err)
	}
	if img.Bounds().Dx() != 320 {
		t.Fatalf("width: %d", img.Bounds().Dx())
	}
}

func TestConvertMaxDim(t *testing.T) {
	p := writeTestJPEG(t, 800, 600)
	res, _, err := ConvertFile(p, ConvertOptions{Target: "png", MaxDim: 200})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if res.Width != 200 || res.Height != 150 {
		t.Fatalf("expected 200x150, got %dx%d", res.Width, res.Height)
	}
}

func TestConvertUnknownTarget(t *testing.T) {
	p := writeTestJPEG(t, 100, 100)
	if _, _, err := ConvertFile(p, ConvertOptions{Target: "webp"}); err == nil {
		t.Fatal("expected error for webp target")
	}
	if _, _, err := ConvertFile(p, ConvertOptions{Target: "psd"}); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestConvertBytesRoundTrip(t *testing.T) {
	p := writeTestJPEG(t, 128, 128)
	_, data, err := ConvertFile(p, ConvertOptions{Target: "png"})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	res, out, err := ConvertBytes("src.png", base64.StdEncoding.EncodeToString(data), ConvertOptions{Target: "bmp"})
	if err != nil {
		t.Fatalf("convert bytes: %v", err)
	}
	if res.ToFormat != "bmp" || len(out) == 0 {
		t.Fatalf("bad result: %+v", res)
	}
	if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("decode bmp: %v", err)
	}
}

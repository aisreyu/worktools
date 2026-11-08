package toolbox

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func readZipNames(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	out := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			rc.Close()
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		rc.Close()
		out[f.Name] = buf.String()
	}
	return out
}

func TestWriteZipEntries(t *testing.T) {
	data, err := WriteZip([]ZipEntry{
		{Name: "a.txt", Data: []byte("hello")},
		{Name: "", Data: []byte("skipped")},
		{Name: "sub/b.txt", Data: []byte("world")},
	})
	if err != nil {
		t.Fatalf("WriteZip: %v", err)
	}
	got := readZipNames(t, data)
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d: %v", len(got), got)
	}
	if got["a.txt"] != "hello" || got["sub/b.txt"] != "world" {
		t.Fatalf("unexpected contents: %v", got)
	}
}

func TestZipPathsDedupesBaseNames(t *testing.T) {
	dir := t.TempDir()
	d1 := filepath.Join(dir, "one")
	d2 := filepath.Join(dir, "two")
	if err := os.MkdirAll(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(d2, 0o755); err != nil {
		t.Fatal(err)
	}
	p1 := filepath.Join(d1, "same.txt")
	p2 := filepath.Join(d2, "same.txt")
	if err := os.WriteFile(p1, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := ZipPaths([]string{p1, p2})
	if err != nil {
		t.Fatalf("ZipPaths: %v", err)
	}
	got := readZipNames(t, data)
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d: %v", len(got), got)
	}
	if got["same.txt"] != "first" || got["same_1.txt"] != "second" {
		t.Fatalf("dedupe failed: %v", got)
	}
}

func TestZipDirRecursiveRelative(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "proj")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "deep.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "proj.zip")
	if err := ZipDir(src, out); err != nil {
		t.Fatalf("ZipDir: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := readZipNames(t, raw)
	if got["root.txt"] != "r" || got["nested/deep.txt"] != "d" {
		t.Fatalf("unexpected entries: %v", got)
	}
}

func TestWriteFileBytesCreatesParents(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "out.bin")
	if err := WriteFileBytes(target, []byte("xyz")); err != nil {
		t.Fatalf("WriteFileBytes: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "xyz" {
		t.Fatalf("got %q", got)
	}
	if err := WriteFileBytes("  ", []byte("x")); err == nil {
		t.Fatal("expected error for empty path")
	}
}

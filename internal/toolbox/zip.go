package toolbox

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteFileBytes writes raw bytes to path, creating parent directories as
// needed. Shared by the service layer for ZIP exports.
func WriteFileBytes(path string, data []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// ZipEntry is a single file to include in an archive.
type ZipEntry struct {
	// Name is the path recorded inside the archive (forward slashes).
	Name string
	// Data is the raw file contents.
	Data []byte
}

// WriteZip encodes the entries as a ZIP archive (deflate) and returns the bytes.
func WriteZip(entries []ZipEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		name = filepath.ToSlash(name)
		w, err := zw.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create zip entry: %w", err)
		}
		if _, err := w.Write(e.Data); err != nil {
			return nil, fmt.Errorf("write zip entry: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// ZipPaths reads the given files from disk into a ZIP archive. Each entry keeps
// only its base name; duplicate base names are disambiguated with a numeric
// suffix so nothing is silently overwritten.
func ZipPaths(paths []string) ([]byte, error) {
	entries := make([]ZipEntry, 0, len(paths))
	used := make(map[string]int, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.Base(p), err)
		}
		name := filepath.Base(p)
		if n := used[name]; n > 0 {
			ext := filepath.Ext(name)
			base := strings.TrimSuffix(name, ext)
			name = fmt.Sprintf("%s_%d%s", base, n, ext)
		}
		used[filepath.Base(p)]++
		entries = append(entries, ZipEntry{Name: name, Data: data})
	}
	return WriteZip(entries)
}

// ZipDir walks dir recursively and packages every regular file into an archive,
// preserving paths relative to dir. The archive is written to outPath.
func ZipDir(dir, outPath string) error {
	root := filepath.Clean(dir)
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("read folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是文件夹", dir)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	var walkErr error
	// Collect first so the archive is written in a stable order.
	var files []string
	walkErr = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if walkErr != nil {
		zw.Close()
		return fmt.Errorf("扫描文件夹失败: %w", walkErr)
	}
	sort.Strings(files)

	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			zw.Close()
			return fmt.Errorf("计算相对路径失败: %w", err)
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			zw.Close()
			return fmt.Errorf("create zip entry: %w", err)
		}
		src, err := os.Open(path)
		if err != nil {
			zw.Close()
			return fmt.Errorf("open %s: %w", rel, err)
		}
		if _, err := io.Copy(w, src); err != nil {
			src.Close()
			zw.Close()
			return fmt.Errorf("write %s: %w", rel, err)
		}
		src.Close()
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	return nil
}

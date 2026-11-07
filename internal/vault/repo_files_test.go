package vault

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/aisreyu/worktools/internal/store"
)

func TestFileEntryInlineRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	body := base64.StdEncoding.EncodeToString([]byte("hello id card"))
	id, err := v.SaveFile(store.FileEntry{Title: "ID", Category: "id", Mime: "image/png", Size: 12, Data: body})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := v.GetFile(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Data != body {
		t.Fatalf("body mismatch: %q", got.Data)
	}
	if got.Title != "ID" || got.Category != "id" {
		t.Fatalf("meta mismatch: %+v", got)
	}

	// Metadata-only update must preserve the stored body.
	if _, err := v.SaveFile(store.FileEntry{ID: id, Title: "ID copy", Category: "licence", Note: "n"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = v.GetFile(id)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Data != body {
		t.Fatal("body should be preserved on metadata update")
	}
	if got.Title != "ID copy" {
		t.Fatalf("title not updated: %q", got.Title)
	}

	list, err := v.ListFiles()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if list[0].Data != "" {
		t.Fatal("List must not return bodies")
	}

	if err := v.DeleteFile(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := v.ListFiles(); len(list) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(list))
	}
}

func TestFileEntryDiskRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// A body larger than inlineMaxBytes is spilled to an encrypted file.
	big := strings.Repeat("A", inlineMaxBytes+1024)
	body := base64.StdEncoding.EncodeToString([]byte(big))
	id, err := v.SaveFile(store.FileEntry{Title: "licence", Mime: "application/pdf", Size: int64(len(big)), Data: body})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := v.GetFile(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Data != body {
		t.Fatal("disk body mismatch")
	}
	if err := v.DeleteFile(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestFileEntryRequiresBody(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := v.SaveFile(store.FileEntry{Title: "x"}); err != ErrEmptyFile {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
	}
}

func TestFileEntryLocked(t *testing.T) {
	v, _ := openTestVault(t)
	if _, err := v.ListFiles(); err != ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
}

package vault

import (
	"testing"

	"github.com/aisreyu/worktools/internal/store"
)

func TestCategoryDefaultsSeededOnUnlock(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	cats, err := v.ListCategories()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cats) != len(defaultCategories) {
		t.Fatalf("expected %d seeded categories, got %d", len(defaultCategories), len(cats))
	}
	for i, c := range cats {
		if c.Code != defaultCategories[i].code || c.Name != defaultCategories[i].name {
			t.Fatalf("category %d mismatch: %+v", i, c)
		}
	}

	// Re-locking and unlocking must not duplicate the seed.
	v.Lock()
	if err := v.Unlock("correct horse battery"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	cats, err = v.ListCategories()
	if err != nil {
		t.Fatalf("list after unlock: %v", err)
	}
	if len(cats) != len(defaultCategories) {
		t.Fatalf("re-seeded on unlock: got %d", len(cats))
	}
}

func TestCategoryCRUD(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}

	cat, err := v.CreateCategory("企业主体")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cat.Code == "" || cat.Name != "企业主体" {
		t.Fatalf("unexpected created category: %+v", cat)
	}

	// The stored name must be ciphertext, not the plaintext label.
	var ct []byte
	if err := v.db.QueryRow(`SELECT name_ct FROM file_categories WHERE id = ?`, cat.ID).Scan(&ct); err != nil {
		t.Fatalf("read name_ct: %v", err)
	}
	if string(ct) == "企业主体" {
		t.Fatal("category name was not encrypted at rest")
	}

	if err := v.RenameCategory(cat.ID, "企业主体（新）"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	cats, err := v.ListCategories()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found bool
	for _, c := range cats {
		if c.ID == cat.ID {
			found = true
			if c.Name != "企业主体（新）" {
				t.Fatalf("rename not persisted: %+v", c)
			}
			if c.Code != cat.Code {
				t.Fatalf("code changed on rename: %q -> %q", cat.Code, c.Code)
			}
		}
	}
	if !found {
		t.Fatal("renamed category missing from list")
	}

	// A category still holding a file cannot be deleted.
	if _, err := v.SaveFile(store.FileEntry{Title: "licence", Category: cat.Code, Data: "aGk="}); err != nil {
		t.Fatalf("save file: %v", err)
	}
	if err := v.DeleteCategory(cat.ID); err != ErrCategoryInUse {
		t.Fatalf("expected ErrCategoryInUse, got %v", err)
	}
	_ = v.DeleteFile(1)
	if err := v.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("delete after file removed: %v", err)
	}
}

func TestCategoryBackupRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	cat, err := v.CreateCategory("企业主体")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := v.SaveFile(store.FileEntry{Title: "doc", Category: cat.Code, Data: "aGk="}); err != nil {
		t.Fatalf("save file: %v", err)
	}

	data, err := v.ExportBackup("backup password")
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	// Restore into a fresh vault and confirm the category survived with its code.
	v2, _ := openTestVault(t)
	if err := v2.Init("another password"); err != nil {
		t.Fatalf("init v2: %v", err)
	}
	if _, err := v2.ImportBackup(data, "backup password"); err != nil {
		t.Fatalf("import: %v", err)
	}
	cats, err := v2.ListCategories()
	if err != nil {
		t.Fatalf("list v2: %v", err)
	}
	var found bool
	for _, c := range cats {
		if c.Code == cat.Code && c.Name == "企业主体" {
			found = true
		}
	}
	if !found {
		t.Fatalf("imported category missing: %+v", cats)
	}
}

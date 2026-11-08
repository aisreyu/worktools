package vault

import (
	"database/sql"
	"testing"
)

func TestAppSecretRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}

	if v.HasAppSecret(AppSecretKeyAutoBackup) {
		t.Fatal("secret should be absent before it is set")
	}

	if err := v.SetAppSecret(AppSecretKeyAutoBackup, "backup-pw-123"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !v.HasAppSecret(AppSecretKeyAutoBackup) {
		t.Fatal("secret should be present after set")
	}

	got, err := v.GetAppSecret(AppSecretKeyAutoBackup)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "backup-pw-123" {
		t.Fatalf("got %q want %q", got, "backup-pw-123")
	}

	// Overwriting the same key must replace the value, not duplicate it.
	if err := v.SetAppSecret(AppSecretKeyAutoBackup, "new-pw"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if got, _ := v.GetAppSecret(AppSecretKeyAutoBackup); got != "new-pw" {
		t.Fatalf("after overwrite got %q want %q", got, "new-pw")
	}

	if err := v.DeleteAppSecret(AppSecretKeyAutoBackup); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if v.HasAppSecret(AppSecretKeyAutoBackup) {
		t.Fatal("secret should be gone after delete")
	}
	if _, err := v.GetAppSecret(AppSecretKeyAutoBackup); err != sql.ErrNoRows {
		t.Fatalf("get after delete: got %v want sql.ErrNoRows", err)
	}
}

func TestAppSecretLocked(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := v.SetAppSecret(AppSecretKeyAutoBackup, "backup-pw"); err != nil {
		t.Fatalf("set: %v", err)
	}

	v.Lock()

	// Reading or writing while locked must surface ErrLocked, not a raw SQL error.
	if _, err := v.GetAppSecret(AppSecretKeyAutoBackup); err != ErrLocked {
		t.Fatalf("get while locked: got %v want ErrLocked", err)
	}
	if err := v.SetAppSecret(AppSecretKeyAutoBackup, "x"); err != ErrLocked {
		t.Fatalf("set while locked: got %v want ErrLocked", err)
	}
	if err := v.DeleteAppSecret(AppSecretKeyAutoBackup); err != ErrLocked {
		t.Fatalf("delete while locked: got %v want ErrLocked", err)
	}
	// HasAppSecret only inspects key presence and does not decrypt, so it stays
	// usable while locked.
	if !v.HasAppSecret(AppSecretKeyAutoBackup) {
		t.Fatal("HasAppSecret should work while locked (no decryption)")
	}

	if err := v.Unlock("correct horse battery"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if got, _ := v.GetAppSecret(AppSecretKeyAutoBackup); got != "backup-pw" {
		t.Fatalf("value lost across lock/unlock: got %q", got)
	}
}

package vault

import (
	"database/sql"
	"testing"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
)

// openTestVault returns an in-memory vault plus its database handle.
func openTestVault(t *testing.T) (*Vault, *sql.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db), db
}

func TestInitUnlockLock(t *testing.T) {
	v, _ := openTestVault(t)

	ok, err := v.Initialised()
	if err != nil || ok {
		t.Fatalf("fresh vault: initialised=%v err=%v", ok, err)
	}
	if err := v.Init("correct horse battery"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if v.Locked() {
		t.Fatal("vault should be unlocked right after init")
	}

	v.Lock()
	if !v.Locked() {
		t.Fatal("expected locked after Lock")
	}

	if err := v.Unlock("wrong password"); err != ErrBadPassword {
		t.Fatalf("wrong password: got %v want ErrBadPassword", err)
	}
	if err := v.Unlock("correct horse battery"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}

func TestShortPasswordRejected(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("short"); err != ErrShortPassword {
		t.Fatalf("got %v want ErrShortPassword", err)
	}
}

func TestURLRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("master-password-1"); err != nil {
		t.Fatalf("init: %v", err)
	}

	id, err := v.SaveURL(store.URLEntry{
		Title: "prod console", URL: "https://example.com", Env: "prod",
		Username: "admin", Password: "s3cr3t-p@ss", Note: "n",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	list, err := v.ListURLs()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Password != "s3cr3t-p@ss" {
		t.Fatalf("unexpected round trip: %+v", list)
	}

	if err := v.DeleteURL(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ = v.ListURLs(); len(list) != 0 {
		t.Fatalf("expected empty after delete")
	}
}

func TestSecretLockedErrors(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("master-password-1"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := v.SaveSecret(store.SecretEntry{Title: "srv", Password: "p", Extra: "key"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	v.Lock()
	if _, err := v.ListSecrets(); err != ErrLocked {
		t.Fatalf("locked list: got %v want ErrLocked", err)
	}
}

func TestOTPRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("master-password-1"); err != nil {
		t.Fatalf("init: %v", err)
	}
	id, err := v.SaveOTP(store.OTPEntry{
		Title: "GitHub", Account: "user@example.com",
		Secret: "JBSWY3DPEHPK3PXP", Algorithm: "SHA1", Digits: 6, Period: 30, Type: "totp",
	})
	if err != nil {
		t.Fatalf("save otp: %v", err)
	}
	got, err := v.GetOTP(id)
	if err != nil {
		t.Fatalf("get otp: %v", err)
	}
	if got.Secret != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("secret round trip: %q", got.Secret)
	}
	if _, err := v.BumpOTPCounter(id); err != nil {
		t.Fatalf("bump counter: %v", err)
	}
}

func TestNoteRoundTrip(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("master-password-1"); err != nil {
		t.Fatalf("init: %v", err)
	}
	id, err := v.SaveNote(store.NoteEntry{Title: "runbook", Content: "restart nginx", Pinned: true})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	notes, err := v.ListNotes()
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 || notes[0].Content != "restart nginx" || !notes[0].Pinned {
		t.Fatalf("unexpected notes: %+v", notes)
	}
	// Update
	if _, err = v.SaveNote(store.NoteEntry{ID: id, Title: "runbook", Content: "updated"}); err != nil {
		t.Fatalf("update note: %v", err)
	}
	got, err := v.GetNote(id)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if got.Content != "updated" || got.Pinned {
		t.Fatalf("unexpected updated note: %+v", got)
	}
	if err := v.DeleteNote(id); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if notes, _ = v.ListNotes(); len(notes) != 0 {
		t.Fatalf("expected empty after delete")
	}
}

func TestRecoveryResetPassword(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("master-password-1"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := v.SaveURL(store.URLEntry{Title: "t", URL: "https://x", Username: "u", Password: "p"}); err != nil {
		t.Fatalf("save url: %v", err)
	}
	qs := []store.RecoveryQuestion{
		{Question: "你的宠物叫什么？", Answer: "Tom"},
		{Question: "你的出生城市？", Answer: "Beijing"},
	}
	if err := v.SetRecoveryQuestions(qs); err != nil {
		t.Fatalf("set recovery: %v", err)
	}

	// Questions readable while locked.
	v.Lock()
	got, err := v.RecoveryQuestions()
	if err != nil || len(got) != 2 {
		t.Fatalf("questions while locked: %v %v", got, err)
	}

	// Wrong answers rejected.
	if err := v.ResetPasswordWithRecovery([]string{"tom", "shanghai"}, "new-password-1"); err != ErrBadRecovery {
		t.Fatalf("wrong answers: got %v want ErrBadRecovery", err)
	}

	// Right answers reset the password (case-insensitive, trimmed).
	if err := v.ResetPasswordWithRecovery([]string{" tom ", "BEIJING"}, "new-password-1"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// Unlock with the new password; data intact.
	if err := v.Unlock("new-password-1"); err != nil {
		t.Fatalf("unlock with new password: %v", err)
	}
	urls, _ := v.ListURLs()
	if len(urls) != 1 || urls[0].Password != "p" {
		t.Fatalf("data after reset: %+v", urls)
	}
}

func TestBackupExportImport(t *testing.T) {
	v, _ := openTestVault(t)
	if err := v.Init("master-password-1"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := v.SaveURL(store.URLEntry{Title: "t", URL: "https://x", Username: "u", Password: "p"}); err != nil {
		t.Fatalf("save url: %v", err)
	}
	if _, err := v.SaveNote(store.NoteEntry{Title: "n", Content: "hello"}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	data, err := v.ExportBackup("backup-pass-1")
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	// Wrong password rejected.
	if _, err := v.ImportBackup(data, "wrong-pass"); err != ErrBadBackup {
		t.Fatalf("wrong password: got %v want ErrBadBackup", err)
	}

	// Correct password merges entries.
	n, err := v.ImportBackup(data, "backup-pass-1")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 2 {
		t.Fatalf("imported %d, want 2", n)
	}
	urls, _ := v.ListURLs()
	if len(urls) != 2 {
		t.Fatalf("urls after import: %d", len(urls))
	}
	notes, _ := v.ListNotes()
	if len(notes) != 2 {
		t.Fatalf("notes after import: %d", len(notes))
	}
}

-- notebook schema (v1)

-- vault_meta holds exactly one row describing how to unlock the database.
CREATE TABLE IF NOT EXISTS vault_meta (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    version       INTEGER NOT NULL,
    kdf           TEXT    NOT NULL,
    kdf_salt      BLOB    NOT NULL,
    kdf_params    TEXT    NOT NULL,
    wrap_cipher   BLOB    NOT NULL,   -- DEK encrypted with the master key
    wrap_nonce    BLOB    NOT NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

-- M1: common URLs together with their credentials.
CREATE TABLE IF NOT EXISTS url_entries (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    title          TEXT    NOT NULL,
    url            TEXT    NOT NULL,
    env            TEXT    NOT NULL DEFAULT '',
    tags           TEXT    NOT NULL DEFAULT '',
    username       TEXT    NOT NULL DEFAULT '',
    password_ct    BLOB,
    password_nonce BLOB,
    note           TEXT    NOT NULL DEFAULT '',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_url_env ON url_entries(env);

-- M2: personal password vault (servers, databases, cloud keys, ...).
CREATE TABLE IF NOT EXISTS secret_entries (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    title          TEXT    NOT NULL,
    category       TEXT    NOT NULL DEFAULT 'other',
    host           TEXT    NOT NULL DEFAULT '',
    port           INTEGER NOT NULL DEFAULT 0,
    username       TEXT    NOT NULL DEFAULT '',
    password_ct    BLOB    NOT NULL,
    password_nonce BLOB    NOT NULL,
    extra_ct       BLOB,
    extra_nonce    BLOB,
    note           TEXT    NOT NULL DEFAULT '',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_secret_category ON secret_entries(category);

-- M3: OTP entries (TOTP / HOTP).
CREATE TABLE IF NOT EXISTS otp_entries (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT    NOT NULL,
    issuer       TEXT    NOT NULL DEFAULT '',
    account      TEXT    NOT NULL DEFAULT '',
    secret_ct    BLOB    NOT NULL,
    secret_nonce BLOB    NOT NULL,
    algorithm    TEXT    NOT NULL DEFAULT 'SHA1',
    digits       INTEGER NOT NULL DEFAULT 6,
    period       INTEGER NOT NULL DEFAULT 30,
    otp_type     TEXT    NOT NULL DEFAULT 'totp',
    counter      INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

-- M4: personal notes (multiple records).
CREATE TABLE IF NOT EXISTS note_entries (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT    NOT NULL,
    content_ct   BLOB,
    content_nonce BLOB,
    pinned       INTEGER NOT NULL DEFAULT 0,
    sensitive    INTEGER NOT NULL DEFAULT 0,
    attach_ct    BLOB,
    attach_nonce BLOB,
    attach_name  TEXT    NOT NULL DEFAULT '',
    attach_mime  TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_note_updated ON note_entries(updated_at DESC);

-- M5: security questions used to reset a forgotten master password.
-- Answers are encrypted with the DEK; the recovery wrap stores a second copy
-- of the DEK wrapped by a key derived from the answers, so a locked vault can
-- still be recovered without the master password.
CREATE TABLE IF NOT EXISTS recovery_meta (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    questions      TEXT    NOT NULL,          -- JSON array of question texts
    answers_ct     BLOB    NOT NULL,          -- JSON array of encrypted answers
    answers_nonce  BLOB    NOT NULL,
    rec_salt       BLOB    NOT NULL,          -- KDF salt for the recovery key
    rec_params     TEXT    NOT NULL,          -- KDF params JSON
    rec_wrap_cipher BLOB   NOT NULL,          -- DEK wrapped by recovery key
    rec_wrap_nonce BLOB    NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

-- M6: schedule entries with due-time reminders.
CREATE TABLE IF NOT EXISTS schedule_entries (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT    NOT NULL,
    detail_ct    BLOB,
    detail_nonce BLOB,
    due_at       INTEGER NOT NULL,     -- unix seconds
    remind       INTEGER NOT NULL DEFAULT 1,  -- minutes before due; 0 = no remind
    done         INTEGER NOT NULL DEFAULT 0,
    notified     INTEGER NOT NULL DEFAULT 0,  -- reminder already fired
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_schedule_due ON schedule_entries(due_at);

-- M7: private-folder files (ID copies, business licences, ...). The file body
-- is stored either inline (small files) as an encrypted blob, or on disk
-- (large files) under a per-user directory, also encrypted. disk_path points at
-- the on-disk file and is only meaningful when stored = 'disk'.
CREATE TABLE IF NOT EXISTS file_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    category    TEXT    NOT NULL DEFAULT 'other',
    note        TEXT    NOT NULL DEFAULT '',
    mime        TEXT    NOT NULL DEFAULT '',
    size        INTEGER NOT NULL DEFAULT 0,
    stored      TEXT    NOT NULL DEFAULT 'db',   -- 'db' | 'disk'
    data_ct     BLOB,
    data_nonce  BLOB,
    disk_path   TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_file_category ON file_entries(category);

-- M8: user-managed categories for the private folder. The display name is
-- encrypted at rest under file_categories:name; code is a stable slug that
-- file_entries.category matches so renaming a category keeps existing files.
CREATE TABLE IF NOT EXISTS file_categories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    code        TEXT    NOT NULL,
    name_ct     BLOB    NOT NULL,
    name_nonce  BLOB    NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_file_categories_code ON file_categories(code);

-- M9: clipboard history captured from the OS clipboard (CopyLess-style). The
-- captured text is encrypted at rest under clipboard_items:text; kind is a
-- coarse label (link | code | text) used for filtering and search.
CREATE TABLE IF NOT EXISTS clipboard_items (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    text_ct    BLOB    NOT NULL,
    text_nonce BLOB    NOT NULL,
    kind       TEXT    NOT NULL DEFAULT 'text',
    pinned     INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_clipboard_created ON clipboard_items(created_at DESC);

-- M9: encrypted key/value settings (currently the clipboard auto-capture switch).
CREATE TABLE IF NOT EXISTS app_settings (
    key        TEXT PRIMARY KEY,
    value_ct   BLOB NOT NULL,
    value_nonce BLOB NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Schema version tracking for future migrations.
CREATE TABLE IF NOT EXISTS app_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO app_meta(key, value) VALUES ('schema_version', '1');

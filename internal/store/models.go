package store

// URL/credential entry (M1). Password holds plaintext only in memory while a
// vault is unlocked; it is encrypted before it ever touches disk.
type URLEntry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Env       string `json:"env"`
	Tags      string `json:"tags"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// SecretEntry is a personal password vault item (M2).
type SecretEntry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	Extra     string `json:"extra,omitempty"` // private key, AK/SK, ...
	Note      string `json:"note"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// OTPEntry is a TOTP/HOTP configuration (M3).
type OTPEntry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Issuer    string `json:"issuer"`
	Account   string `json:"account"`
	Secret    string `json:"secret,omitempty"` // Base32, plaintext in memory only
	Algorithm string `json:"algorithm"`        // SHA1 | SHA256 | SHA512
	Digits    int    `json:"digits"`
	Period    int    `json:"period"`
	Type      string `json:"type"` // totp | hotp
	Counter   int64  `json:"counter"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// NoteEntry is a personal note (M4). Content is encrypted at rest.
type NoteEntry struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Pinned  bool   `json:"pinned"`
	// Sensitive marks a note whose body must only ever be written through the
	// vault (encrypted) channel, never shown in plaintext listings.
	Sensitive bool `json:"sensitive"`
	// Attachment holds a base64 image pasted/dropped into the note. It is
	// encrypted at rest alongside the content.
	Attachment     string `json:"attachment,omitempty"`
	AttachmentName string `json:"attachmentName,omitempty"`
	AttachmentMime string `json:"attachmentMime,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// FileEntry is a private-folder document (M7). Data holds the base64 file body
// and is only populated on demand (Get); List returns metadata only because
// large files are kept on disk and decrypted into Data when requested.
type FileEntry struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Note     string `json:"note"`
	Mime     string `json:"mime"`
	Size     int64  `json:"size"`
	// Data is base64 of the file body. On save it carries the new content; on
	// get it carries the decrypted content.
	Data      string `json:"data,omitempty"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// FileCategory is a user-managed private-folder category (M8). Name is the
// decrypted display label; Code is a stable slug that file_entries.category
// matches, so renaming a category never orphans its files.
type FileCategory struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

// ClipboardItem is one captured clipboard entry (M9). Text is the decrypted
// capture; it is encrypted at rest under clipboard_items:text. Kind is a coarse
// label (link | code | text) used for filtering.
type ClipboardItem struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	Preview   string `json:"preview"`
	Kind      string `json:"kind"`
	Pinned    bool   `json:"pinned"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// RecoveryQuestion is one security question; the answer never leaves Go.
type RecoveryQuestion struct {
	Question string `json:"question"`
	Answer   string `json:"answer,omitempty"`
}

// ScheduleEntry is a calendar schedule item with a due time (M6). Detail is
// encrypted at rest; title stays plaintext for listing while locked-out views
// are impossible anyway (service checks the vault).
type ScheduleEntry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	DueAt     int64  `json:"dueAt"`  // unix seconds
	Remind    int    `json:"remind"` // minutes before due; 0 = off
	Done      bool   `json:"done"`
	Notified  bool   `json:"notified"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

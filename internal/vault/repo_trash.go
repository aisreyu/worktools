package vault

import (
	"errors"
	"os"
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// errBadTrashKind is returned when an unknown entity kind is passed in.
var errBadTrashKind = errors.New("unknown trash item kind")

// ---- soft-delete (recycle bin) ----

// trashSpec describes one entity table that supports soft deletion.
type trashSpec struct {
	kind   string
	table  string
	title  string // column holding the display title
	sub    string // column holding a subtitle (may be "" for notes)
}

var trashSpecs = []trashSpec{
	{kind: "url", table: "url_entries", title: "title", sub: "url"},
	{kind: "secret", table: "secret_entries", title: "title", sub: "host"},
	{kind: "otp", table: "otp_entries", title: "title", sub: "issuer"},
	{kind: "note", table: "note_entries", title: "title", sub: ""},
	{kind: "file", table: "file_entries", title: "title", sub: "category"},
}

// ListTrash returns every soft-deleted entry across the vault, newest first.
// Titles/subtitles are read directly (titles are plaintext columns; notes have
// no encrypted subtitle). This works while the vault is unlocked.
func (v *Vault) ListTrash() ([]store.TrashItem, error) {
	if _, err := v.cryptoOrErr(); err != nil {
		return nil, err
	}
	var out []store.TrashItem
	for _, sp := range trashSpecs {
		cols := "id, " + sp.title
		if sp.sub != "" {
			cols += ", " + sp.sub
		}
		rows, err := v.db.Query(`SELECT ` + cols + `, deleted_at FROM ` + sp.table + ` WHERE deleted_at != 0 ORDER BY deleted_at DESC`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				item store.TrashItem
				sub  string
			)
			if sp.sub != "" {
				if err := rows.Scan(&item.ID, &item.Title, &sub, &item.DeletedAt); err != nil {
					rows.Close()
					return nil, err
				}
			} else {
				if err := rows.Scan(&item.ID, &item.Title, &item.DeletedAt); err != nil {
					rows.Close()
					return nil, err
				}
			}
			item.Kind = sp.kind
			item.Subtitle = sub
			out = append(out, item)
		}
		rows.Close()
	}
	// Sort newest-deleted first across all kinds (small lists; insertion sort).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].DeletedAt > out[j-1].DeletedAt; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

func specFor(kind string) (trashSpec, bool) {
	for _, sp := range trashSpecs {
		if sp.kind == kind {
			return sp, true
		}
	}
	return trashSpec{}, false
}

// Restore puts a trashed entry back into its normal list.
func (v *Vault) Restore(kind string, id int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	sp, ok := specFor(kind)
	if !ok {
		return errBadTrashKind
	}
	_, err := v.db.Exec(`UPDATE `+sp.table+` SET deleted_at = 0 WHERE id = ?`, id)
	return err
}

// Purge permanently deletes a single trashed entry. For files it also removes
// the on-disk blob if the body was stored on disk.
func (v *Vault) Purge(kind string, id int64) error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	sp, ok := specFor(kind)
	if !ok {
		return errBadTrashKind
	}
	if kind == "file" {
		var mode, path string
		if err := v.db.QueryRow(`SELECT stored, disk_path FROM file_entries WHERE id = ?`, id).Scan(&mode, &path); err == nil {
			defer func() {
				if mode == "disk" {
					removeIfSet(path)
				}
			}()
		}
	}
	_, err := v.db.Exec(`DELETE FROM `+sp.table+` WHERE id = ? AND deleted_at != 0`, id)
	return err
}

// EmptyTrash permanently removes every soft-deleted entry, cleaning up any
// on-disk file blobs first.
func (v *Vault) EmptyTrash() error {
	if _, err := v.cryptoOrErr(); err != nil {
		return err
	}
	// Remove disk-backed file bodies before dropping their rows.
	rows, err := v.db.Query(`SELECT disk_path FROM file_entries WHERE deleted_at != 0 AND stored = 'disk'`)
	if err == nil {
		var paths []string
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				paths = append(paths, p)
			}
		}
		rows.Close()
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}
	for _, sp := range trashSpecs {
		if _, err := v.db.Exec(`DELETE FROM ` + sp.table + ` WHERE deleted_at != 0`); err != nil {
			return err
		}
	}
	return nil
}

// PurgeTrashOlderThan permanently removes entries that were deleted more than
// the given number of seconds ago. Returns the number of entries purged.
func (v *Vault) PurgeTrashOlderThan(seconds int64) (int64, error) {
	if _, err := v.cryptoOrErr(); err != nil {
		return 0, err
	}
	if seconds <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Unix() - seconds
	// Remove disk-backed file bodies that will be pruned, before dropping rows.
	if rows, err := v.db.Query(`SELECT disk_path FROM file_entries WHERE deleted_at != 0 AND deleted_at < ? AND stored = 'disk'`, cutoff); err == nil {
		var paths []string
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				paths = append(paths, p)
			}
		}
		rows.Close()
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}
	var total int64
	for _, sp := range trashSpecs {
		res, err := v.db.Exec(`DELETE FROM `+sp.table+` WHERE deleted_at != 0 AND deleted_at < ?`, cutoff)
		if err != nil {
			return total, err
		}
		if n, err := res.RowsAffected(); err == nil {
			total += n
		}
	}
	return total, nil
}

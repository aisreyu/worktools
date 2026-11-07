package vault

import (
	"database/sql"
	"time"

	"github.com/aisreyu/worktools/internal/store"
)

// ---- M6: schedule entries ----

const aadScheduleDetail = "schedule_entries:detail"

// ListSchedules returns all schedules, pending first then by due time.
func (v *Vault) ListSchedules() ([]store.ScheduleEntry, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := v.db.Query(`SELECT id, title, detail_ct, detail_nonce, due_at, remind, done, notified, created_at, updated_at
        FROM schedule_entries ORDER BY done ASC, due_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.ScheduleEntry
	for rows.Next() {
		var (
			e                store.ScheduleEntry
			ct, non          []byte
			done, notified   int
		)
		if err := rows.Scan(&e.ID, &e.Title, &ct, &non, &e.DueAt, &e.Remind, &done, &notified, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		if ct != nil {
			if e.Detail, err = c.DecryptField(ct, non, aadScheduleDetail); err != nil {
				return nil, err
			}
		}
		e.Done = done != 0
		e.Notified = notified != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveSchedule inserts (ID==0) or updates a schedule.
func (v *Vault) SaveSchedule(e store.ScheduleEntry) (int64, error) {
	c, err := v.cryptoOrErr()
	if err != nil {
		return 0, err
	}
	var ct, non []byte
	if e.Detail != "" {
		ct, non, err = c.EncryptField(e.Detail, aadScheduleDetail)
		if err != nil {
			return 0, err
		}
	}
	done, notified := 0, 0
	if e.Done {
		done = 1
	}
	if e.Notified {
		notified = 1
	}
	now := time.Now().Unix()
	if e.ID == 0 {
		res, err := v.db.Exec(`INSERT INTO schedule_entries
            (title, detail_ct, detail_nonce, due_at, remind, done, notified, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, ct, non, e.DueAt, e.Remind, done, notified, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = v.db.Exec(`UPDATE schedule_entries SET title=?, detail_ct=?, detail_nonce=?, due_at=?, remind=?, done=?, notified=?, updated_at=? WHERE id=?`,
		e.Title, ct, non, e.DueAt, e.Remind, done, notified, now, e.ID)
	return e.ID, err
}

// DeleteSchedule removes a schedule.
func (v *Vault) DeleteSchedule(id int64) error {
	_, err := v.db.Exec(`DELETE FROM schedule_entries WHERE id = ?`, id)
	return err
}

// SetScheduleDone flips the done flag.
func (v *Vault) SetScheduleDone(id int64, done bool) error {
	d := 0
	if done {
		d = 1
	}
	_, err := v.db.Exec(`UPDATE schedule_entries SET done=?, updated_at=? WHERE id=?`, d, time.Now().Unix(), id)
	return err
}

// MarkScheduleNotified records that the reminder fired (so it fires once).
func (v *Vault) MarkScheduleNotified(id int64) error {
	_, err := v.db.Exec(`UPDATE schedule_entries SET notified=1, updated_at=? WHERE id=?`, time.Now().Unix(), id)
	return err
}

// scheduleRow mirrors schedule_entries for backup/restore.
type scheduleRow struct {
	ID        int64
	Title     string
	Detail    string
	DueAt     int64
	Remind    int
	Done      bool
	CreatedAt int64
	UpdatedAt int64
}

// listScheduleRows returns raw decrypted schedules for backup.
func (v *Vault) listScheduleRows() ([]scheduleRow, error) {
	entries, err := v.ListSchedules()
	if err != nil {
		return nil, err
	}
	out := make([]scheduleRow, 0, len(entries))
	for _, e := range entries {
		out = append(out, scheduleRow{e.ID, e.Title, e.Detail, e.DueAt, e.Remind, e.Done, e.CreatedAt, e.UpdatedAt})
	}
	return out, nil
}

// restoreSchedules inserts backup rows (merge mode).
func (v *Vault) restoreSchedules(rows []scheduleRow) error {
	for _, r := range rows {
		if _, err := v.SaveSchedule(store.ScheduleEntry{
			Title: r.Title, Detail: r.Detail, DueAt: r.DueAt, Remind: r.Remind,
			Done: r.Done, CreatedAt: r.CreatedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

// dueSchedules returns pending schedules whose reminder time has arrived but
// has not fired yet: now >= due_at - remind*60.
func (v *Vault) dueSchedules(now time.Time) ([]store.ScheduleEntry, error) {
	all, err := v.ListSchedules()
	if err != nil {
		return nil, err
	}
	var out []store.ScheduleEntry
	for _, e := range all {
		if e.Done || e.Notified {
			continue
		}
		if e.Remind <= 0 {
			continue
		}
		if now.Unix() >= e.DueAt-int64(e.Remind)*60 {
			out = append(out, e)
		}
	}
	return out, nil
}

// DueSchedules is the exported form used by ScheduleService.
func (v *Vault) DueSchedules(now time.Time) ([]store.ScheduleEntry, error) {
	return v.dueSchedules(now)
}

var _ = sql.ErrNoRows // keep import when helpers change

package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/aisreyu/worktools/internal/holiday"
	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// ScheduleService manages calendar schedules with due-time reminders.
type ScheduleService struct {
	v        *vault.Vault
	notifier *notifications.NotificationService
}

// NewScheduleService wires the service to a vault and the desktop notifier.
func NewScheduleService(v *vault.Vault, notifier *notifications.NotificationService) *ScheduleService {
	return &ScheduleService{v: v, notifier: notifier}
}

// Notify delivers a reminder through the OS notification center. Returns an
// error so the frontend can fall back to an in-app message if it fails (for
// example when the app is unsigned or the user denied permission).
func (s *ScheduleService) Notify(title, body string) error {
	if s.notifier == nil {
		return fmt.Errorf("notifications unavailable")
	}
	return s.notifier.SendNotification(notifications.NotificationOptions{
		ID:                fmt.Sprintf("schedule-%d", time.Now().UnixNano()),
		Title:             title,
		Body:              body,
		InterruptionLevel: notifications.InterruptionLevelActive,
	})
}

// List returns all schedules.
func (s *ScheduleService) List() ([]store.ScheduleEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListSchedules()
}

// Save creates or updates a schedule. Remind is minutes before DueAt (0 = off).
func (s *ScheduleService) Save(e store.ScheduleEntry) (int64, error) {
	if strings.TrimSpace(e.Title) == "" {
		return 0, errText("title is required")
	}
	if e.DueAt <= 0 {
		return 0, errText("due time is required")
	}
	if e.Remind < 0 {
		e.Remind = 0
	}
	e.Recur = normalizeRecur(e.Recur)
	return s.v.SaveSchedule(e)
}

// normalizeRecur maps unsupported values to none (one-shot).
func normalizeRecur(r string) string {
	switch r {
	case "daily", "weekly", "monthly":
		return r
	default:
		return ""
	}
}

// nextDue rolls a due timestamp forward by one recurrence period.
func nextDue(dueAt int64, recur string) int64 {
	t := time.Unix(dueAt, 0)
	switch recur {
	case "daily":
		t = t.AddDate(0, 0, 1)
	case "weekly":
		t = t.AddDate(0, 0, 7)
	case "monthly":
		t = t.AddDate(0, 1, 0)
	default:
		return dueAt
	}
	return t.Unix()
}

// Delete removes a schedule.
func (s *ScheduleService) Delete(id int64) error {
	return s.v.DeleteSchedule(id)
}

// SetDone flips the done flag. Marking a recurring schedule done rolls its due
// time forward to the next period and re-arms the reminder instead of closing
// it, so the series keeps firing.
func (s *ScheduleService) SetDone(id int64, done bool) error {
	if done {
		all, err := s.v.ListSchedules()
		if err != nil {
			return err
		}
		for _, e := range all {
			if e.ID == id {
				if e.Recur != "" {
					return s.v.RescheduleNext(id, nextDue(e.DueAt, e.Recur))
				}
				break
			}
		}
	}
	return s.v.SetScheduleDone(id, done)
}

// DueForNotify returns pending schedules whose reminder time has arrived and
// marks them notified so each fires exactly once.
func (s *ScheduleService) DueForNotify() ([]store.ScheduleEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	due, err := s.v.DueSchedules(time.Now())
	if err != nil {
		return nil, err
	}
	for _, e := range due {
		if err := s.v.MarkScheduleNotified(e.ID); err != nil {
			return nil, err
		}
	}
	return due, nil
}

// HolidayMonth returns the statutory holidays and adjusted workdays in the
// month of the given unix timestamp.
func (s *ScheduleService) HolidayMonth(unixSec int64) ([]holiday.Day, error) {
	return holiday.Month(time.Unix(unixSec, 0)), nil
}

// HolidayLookup returns the special-day info for one unix timestamp.
func (s *ScheduleService) HolidayLookup(unixSec int64) (holiday.Day, error) {
	return holiday.Lookup(time.Unix(unixSec, 0)), nil
}

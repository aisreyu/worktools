package services

import (
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/holiday"
	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// ScheduleService manages calendar schedules with due-time reminders.
type ScheduleService struct {
	v *vault.Vault
}

// NewScheduleService wires the service to a vault.
func NewScheduleService(v *vault.Vault) *ScheduleService { return &ScheduleService{v: v} }

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
	return s.v.SaveSchedule(e)
}

// Delete removes a schedule.
func (s *ScheduleService) Delete(id int64) error {
	return s.v.DeleteSchedule(id)
}

// SetDone flips the done flag.
func (s *ScheduleService) SetDone(id int64, done bool) error {
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

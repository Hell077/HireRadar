package domain

import (
	"errors"
	"time"
)

var ErrInvalidPreferences = errors.New("invalid notification preferences")

type Preferences struct {
	Enabled       bool    `json:"enabled"`
	MinimumScore  int     `json:"minimum_score"`
	Immediate     bool    `json:"immediate"`
	DigestEnabled bool    `json:"digest_enabled"`
	Timezone      string  `json:"timezone"`
	QuietStart    *string `json:"quiet_start,omitempty"`
	QuietEnd      *string `json:"quiet_end,omitempty"`
	MaxPerDay     *int    `json:"max_per_day,omitempty"`
}

func DefaultPreferences() Preferences {
	return Preferences{Enabled: true, MinimumScore: 70, Immediate: true, Timezone: "UTC"}
}

func (p Preferences) Validate() error {
	if p.MinimumScore < 0 || p.MinimumScore > 100 || p.Timezone == "" {
		return ErrInvalidPreferences
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil {
		return ErrInvalidPreferences
	}
	if (p.QuietStart == nil) != (p.QuietEnd == nil) {
		return ErrInvalidPreferences
	}
	if p.QuietStart != nil {
		if _, err := time.Parse("15:04", *p.QuietStart); err != nil {
			return ErrInvalidPreferences
		}
		if _, err := time.Parse("15:04", *p.QuietEnd); err != nil || *p.QuietStart == *p.QuietEnd {
			return ErrInvalidPreferences
		}
	}
	if p.MaxPerDay != nil && (*p.MaxPerDay < 1 || *p.MaxPerDay > 100) {
		return ErrInvalidPreferences
	}
	return nil
}

// ScheduleAt moves a delivery that lands in quiet hours to the next quiet-hour end.
func (p Preferences) ScheduleAt(at time.Time) time.Time {
	if p.QuietStart == nil || p.QuietEnd == nil {
		return at
	}
	location, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return at
	}
	local := at.In(location)
	start, _ := time.Parse("15:04", *p.QuietStart)
	end, _ := time.Parse("15:04", *p.QuietEnd)
	minute := local.Hour()*60 + local.Minute()
	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()
	inQuiet := false
	if startMinute < endMinute {
		inQuiet = minute >= startMinute && minute < endMinute
	} else {
		inQuiet = minute >= startMinute || minute < endMinute
	}
	if !inQuiet {
		return at
	}
	endToday := time.Date(local.Year(), local.Month(), local.Day(), end.Hour(), end.Minute(), 0, 0, location)
	if startMinute > endMinute && minute >= startMinute {
		return endToday.AddDate(0, 0, 1).UTC()
	}
	return endToday.UTC()
}

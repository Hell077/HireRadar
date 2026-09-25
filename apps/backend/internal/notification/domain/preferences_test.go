package domain

import (
	"testing"
	"time"
)

func TestScheduleAtOvernightQuietHours(t *testing.T) {
	start, end := "23:00", "08:00"
	prefs := NotificationPreferences{Enabled: true, MinimumScore: 70, Immediate: true, Timezone: "Asia/Almaty", QuietStart: &start, QuietEnd: &end}
	at := time.Date(2026, 9, 25, 18, 30, 0, 0, time.UTC) // 00:30 in Almaty
	want := time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC) // 08:00 in Almaty
	if got := prefs.ScheduleAt(at); !got.Equal(want) {
		t.Fatalf("ScheduleAt() = %s, want %s", got, want)
	}
}

func TestPreferencesValidation(t *testing.T) {
	start, end := "23:00", "08:00"
	prefs := NotificationPreferences{Enabled: true, MinimumScore: 70, Immediate: true, Timezone: "Asia/Almaty", QuietStart: &start, QuietEnd: &end}
	if err := prefs.Validate(); err != nil {
		t.Fatal(err)
	}
	prefs.Timezone = "Mars/Olympus"
	if err := prefs.Validate(); err == nil {
		t.Fatal("invalid timezone was accepted")
	}
}

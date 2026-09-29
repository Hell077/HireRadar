package postgres

import (
	"testing"
	"time"
)

func TestOpenNotificationURLUsesConfiguredAPIBase(t *testing.T) {
	item := dueNotification{ID: "notification-id", ApplyURL: "https://jobs.example/apply"}
	if got := NewWorker(nil, nil, time.Now).openNotificationURL(item); got != item.ApplyURL {
		t.Fatalf("unset base URL = %q, want direct URL", got)
	}
	if got := NewWorker(nil, nil, time.Now, "https://api.example/").openNotificationURL(item); got != "https://api.example/api/v1/notifications/open/notification-id" {
		t.Fatalf("tracked open URL = %q", got)
	}
	if got := NewWorker(nil, nil, time.Now, "javascript:bad").openNotificationURL(item); got != item.ApplyURL {
		t.Fatalf("invalid base URL = %q, want direct URL", got)
	}
}

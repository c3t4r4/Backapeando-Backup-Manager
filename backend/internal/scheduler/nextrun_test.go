package scheduler

import (
	"testing"
	"time"
)

func TestNextRunTime_ValidCron(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := NextRunTime("0 3 * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNextRunTime_InvalidCron(t *testing.T) {
	_, err := NextRunTime("not a cron expression", time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression, got nil")
	}
}

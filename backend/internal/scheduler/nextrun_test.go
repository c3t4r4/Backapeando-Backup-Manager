package scheduler

import (
	"testing"
	"time"
)

func TestNextRunTime_AlwaysInterpretsInSaoPaulo(t *testing.T) {
	// Load the expected timezone
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}

	// Test case 1: from = 2026-06-15 00:00:00 UTC = 2026-06-14 21:00:00 in Sao Paulo (before 3am)
	// The next "0 3 * * *" should be 2026-06-15 06:00:00 UTC = 2026-06-15 03:00:00 in Sao Paulo
	fromUTC := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	got, err := NextRunTime("0 3 * * *", fromUTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify result is correct: should be 06:00 UTC = 03:00 Sao Paulo
	wantUTC := time.Date(2026, 6, 15, 6, 0, 0, 0, time.UTC)
	if !got.Equal(wantUTC) {
		t.Errorf("got %v, want %v (result in Sao Paulo: %v)", got, wantUTC, got.In(loc))
	}

	// Test case 2: from = 2026-06-14 21:30:00 in Sao Paulo (30min before 3am)
	// Input in a different timezone should still produce the same UTC result
	fromBrazil := time.Date(2026, 6, 14, 21, 30, 0, 0, loc)
	got2, err := NextRunTime("0 3 * * *", fromBrazil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be the same as case 1 (next 3am Sao Paulo = 06:00 UTC)
	if !got2.Equal(wantUTC) {
		t.Errorf("got %v, want %v (input from different timezone should yield same result)", got2, wantUTC)
	}

	// Test case 3: from = 2026-06-15 03:00:00 in Sao Paulo (exactly at 3am, should fire tomorrow)
	fromAtThreeSaoPaulo := time.Date(2026, 6, 15, 3, 0, 0, 0, loc)
	got3, err := NextRunTime("0 3 * * *", fromAtThreeSaoPaulo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Next 3am in Sao Paulo = 2026-06-16 06:00 UTC
	wantNext := time.Date(2026, 6, 16, 6, 0, 0, 0, time.UTC)
	if !got3.Equal(wantNext) {
		t.Errorf("got %v, want %v (next 3am Sao Paulo after 3am)", got3, wantNext)
	}
}

func TestNextRunTime_InvalidCron(t *testing.T) {
	_, err := NextRunTime("not a cron expression", time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression, got nil")
	}
}

package handlers

import (
	"testing"
	"time"
)

// TestRecomputeNextRunAtOnCronChange_CronChangedAndAlreadyScheduled is a
// regression test for the gap registered by the Validator in
// docs/RegrasNegocio.md (RN-BACKUP-029, "Caso não previsto") and
// docs/Arquitetura.md ("Pontos a definir"): editing cronExpression on an
// already-scheduled server used to leave the stale next_run_at in place
// until the next scheduler claim.
func TestRecomputeNextRunAtOnCronChange_CronChangedAndAlreadyScheduled(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	current := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)

	got, err := recomputeNextRunAtOnCronChange("0 3 * * *", "0 5 * * *", &current, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected a recomputed next_run_at, got nil")
	}
	// NextRunTime now always interprets in America/Sao_Paulo, so from 2026-01-01 00:00 UTC
	// (= 2026-12-31 21:00 in Sao Paulo), next 5am is 2026-01-01 08:00 UTC (= 05:00 Sao Paulo)
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	want := time.Date(2026, 1, 1, 5, 0, 0, 0, loc).UTC()
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", *got, want)
	}
}

func TestRecomputeNextRunAtOnCronChange_CronUnchangedIsNoop(t *testing.T) {
	current := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)

	got, err := recomputeNextRunAtOnCronChange("0 3 * * *", "0 3 * * *", &current, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (no recompute) when cron did not change, got %v", *got)
	}
}

func TestRecomputeNextRunAtOnCronChange_EmptyNewCronIsNoop(t *testing.T) {
	current := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)

	got, err := recomputeNextRunAtOnCronChange("0 3 * * *", "", &current, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (no recompute) when newCron is empty, got %v", *got)
	}
}

// TestRecomputeNextRunAtOnCronChange_NeverScheduledIsLeftToBootstrap ensures
// this handler-level recompute never takes over the "first time ready"
// bootstrap case (RN-BACKUP-029, bootstrapNextRunAt) — that remains the sole
// writer for a server whose next_run_at is still nil.
func TestRecomputeNextRunAtOnCronChange_NeverScheduledIsLeftToBootstrap(t *testing.T) {
	got, err := recomputeNextRunAtOnCronChange("0 3 * * *", "0 5 * * *", nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil when server was never scheduled, got %v", *got)
	}
}

func TestRecomputeNextRunAtOnCronChange_InvalidCronReturnsError(t *testing.T) {
	current := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)

	got, err := recomputeNextRunAtOnCronChange("0 3 * * *", "not a cron expression", &current, time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression")
	}
	if got != nil {
		t.Errorf("expected nil next_run_at on invalid cron, got %v", *got)
	}
}

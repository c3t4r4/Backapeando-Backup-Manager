package handlers

import (
	"testing"
	"time"

	"backapeando-backup-manager/internal/domain"
)

// TestPivotDestinationBuckets is a regression test for the risk the plan
// itself flagged before implementation ("Zero-fill é obrigatório... Testar
// explicitamente com um destino que só tem dado em 1 dos 30 dias") and for
// an achado do Validator: the end-to-end HTTP test only asserted shapes and
// a single "today" value, never that a sparse destination's other positions
// stay exactly zero, nor the unknown-destination fallback branch.
func TestPivotDestinationBuckets(t *testing.T) {
	targetA := "target-a"
	targetB := "target-b"
	unknownTarget := "target-not-in-destinations"

	destinations := []dashboardDestinationDTO{
		{StorageTargetID: &targetA, Name: "Azure Produção"},
		{StorageTargetID: &targetB, Name: "S3 Backup Externo"},
		{StorageTargetID: nil, Name: "Sem destino"},
	}
	labels := []string{"2026-09-14", "2026-09-15", "2026-09-16"}
	const layout = "2006-01-02"

	mustParse := func(s string) time.Time {
		tm, err := time.Parse(layout, s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return tm
	}

	t.Run("sparse destination stays zero everywhere except its one populated label", func(t *testing.T) {
		buckets := []domain.BackupRunDestinationBucket{
			{Bucket: mustParse("2026-09-15"), StorageTargetID: &targetA, RunCount: 3, TotalBytes: 1000},
		}

		countSeries, bytesSeries := pivotDestinationBuckets(buckets, labels, layout, destinations)

		if len(countSeries) != len(destinations) || len(bytesSeries) != len(destinations) {
			t.Fatalf("len(countSeries)=%d len(bytesSeries)=%d, want %d (one per destination)", len(countSeries), len(bytesSeries), len(destinations))
		}

		wantCount := []int64{0, 3, 0}
		wantBytes := []int64{0, 1000, 0}
		if got := countSeries[0].Data; !equalInt64(got, wantCount) {
			t.Errorf("countSeries[targetA].Data = %v, want %v", got, wantCount)
		}
		if got := bytesSeries[0].Data; !equalInt64(got, wantBytes) {
			t.Errorf("bytesSeries[targetA].Data = %v, want %v", got, wantBytes)
		}
		// The other two destinations (targetB, "Sem destino") had no runs at
		// all — every position must still be present and zero, never absent.
		for i, name := range []string{"targetB", "Sem destino"} {
			idx := i + 1
			if got := countSeries[idx].Data; !equalInt64(got, []int64{0, 0, 0}) {
				t.Errorf("countSeries[%s].Data = %v, want all zero", name, got)
			}
			if got := bytesSeries[idx].Data; !equalInt64(got, []int64{0, 0, 0}) {
				t.Errorf("bytesSeries[%s].Data = %v, want all zero", name, got)
			}
		}
	})

	t.Run("multiple destinations in the same bucket accumulate independently", func(t *testing.T) {
		buckets := []domain.BackupRunDestinationBucket{
			{Bucket: mustParse("2026-09-14"), StorageTargetID: &targetA, RunCount: 1, TotalBytes: 100},
			{Bucket: mustParse("2026-09-14"), StorageTargetID: &targetB, RunCount: 2, TotalBytes: 200},
			{Bucket: mustParse("2026-09-14"), StorageTargetID: nil, RunCount: 5, TotalBytes: 500},
		}

		countSeries, _ := pivotDestinationBuckets(buckets, labels, layout, destinations)

		if got, want := countSeries[0].Data[0], int64(1); got != want {
			t.Errorf("countSeries[targetA].Data[0] = %d, want %d", got, want)
		}
		if got, want := countSeries[1].Data[0], int64(2); got != want {
			t.Errorf("countSeries[targetB].Data[0] = %d, want %d", got, want)
		}
		if got, want := countSeries[2].Data[0], int64(5); got != want {
			t.Errorf("countSeries[Sem destino].Data[0] = %d, want %d", got, want)
		}
	})

	t.Run("two runs in the same bucket and destination are summed, not overwritten", func(t *testing.T) {
		buckets := []domain.BackupRunDestinationBucket{
			{Bucket: mustParse("2026-09-16"), StorageTargetID: &targetA, RunCount: 1, TotalBytes: 100},
			{Bucket: mustParse("2026-09-16"), StorageTargetID: &targetA, RunCount: 1, TotalBytes: 50},
		}

		countSeries, bytesSeries := pivotDestinationBuckets(buckets, labels, layout, destinations)

		if got, want := countSeries[0].Data[2], int64(2); got != want {
			t.Errorf("countSeries[targetA].Data[2] = %d, want %d (summed)", got, want)
		}
		if got, want := bytesSeries[0].Data[2], int64(150); got != want {
			t.Errorf("bytesSeries[targetA].Data[2] = %d, want %d (summed)", got, want)
		}
	})

	t.Run("bucket referencing a destination absent from the destinations list falls back to Sem destino", func(t *testing.T) {
		buckets := []domain.BackupRunDestinationBucket{
			{Bucket: mustParse("2026-09-14"), StorageTargetID: &unknownTarget, RunCount: 7, TotalBytes: 700},
		}

		countSeries, bytesSeries := pivotDestinationBuckets(buckets, labels, layout, destinations)

		noDestinationIdx := len(destinations) - 1
		if got, want := countSeries[noDestinationIdx].Data[0], int64(7); got != want {
			t.Errorf("countSeries[Sem destino].Data[0] = %d, want %d (unknown destination folded in)", got, want)
		}
		if got, want := bytesSeries[noDestinationIdx].Data[0], int64(700); got != want {
			t.Errorf("bytesSeries[Sem destino].Data[0] = %d, want %d (unknown destination folded in)", got, want)
		}
		// Must not have silently created an extra series for the unknown destination.
		if len(countSeries) != len(destinations) {
			t.Errorf("len(countSeries) = %d, want %d (unknown destination must not grow the series list)", len(countSeries), len(destinations))
		}
	})

	t.Run("bucket outside the requested labels is silently ignored, not out-of-range", func(t *testing.T) {
		buckets := []domain.BackupRunDestinationBucket{
			{Bucket: mustParse("2099-01-01"), StorageTargetID: &targetA, RunCount: 9, TotalBytes: 900},
		}

		countSeries, bytesSeries := pivotDestinationBuckets(buckets, labels, layout, destinations)

		if got := countSeries[0].Data; !equalInt64(got, []int64{0, 0, 0}) {
			t.Errorf("countSeries[targetA].Data = %v, want all zero (out-of-range bucket ignored)", got)
		}
		if got := bytesSeries[0].Data; !equalInt64(got, []int64{0, 0, 0}) {
			t.Errorf("bytesSeries[targetA].Data = %v, want all zero (out-of-range bucket ignored)", got)
		}
	})
}

func equalInt64(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

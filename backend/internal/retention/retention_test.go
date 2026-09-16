package retention

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func keptNames(keep map[string]bool) map[string]bool {
	return keep
}

// TestDecide_FewerThanTotal covers "menos de 3 backups no total" — every
// blob must be kept regardless of the policy, since there simply aren't
// enough blobs to ever cross into "not kept" territory.
func TestDecide_FewerThanTotal(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	blobs := []BlobInfo{
		{Name: "a", LastModified: mustParse(t, "2026-09-10T00:00:00Z")},
		{Name: "b", LastModified: mustParse(t, "2026-09-12T00:00:00Z")},
	}
	policy := Policy{RecentCount: 3, MonthlyCount: 12}

	keep := Decide(blobs, policy, now)

	for _, b := range blobs {
		if !keep[b.Name] {
			t.Errorf("expected %q to be kept (fewer blobs than RecentCount), got not kept", b.Name)
		}
	}
	if len(keep) != 2 {
		t.Errorf("expected 2 kept blobs, got %d", len(keep))
	}
}

// TestDecide_MonthsWithGaps covers "meses com lacunas" — the algorithm must
// not invent entries for months that have no backup, and must not break
// when iterating across a gap.
func TestDecide_MonthsWithGaps(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	blobs := []BlobInfo{
		// 3 very recent blobs — always kept via RecentCount.
		{Name: "recent-1", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "recent-2", LastModified: mustParse(t, "2026-09-13T00:00:00Z")},
		{Name: "recent-3", LastModified: mustParse(t, "2026-09-12T00:00:00Z")},
		// July: one backup (June has none — a gap).
		{Name: "july", LastModified: mustParse(t, "2026-07-15T00:00:00Z")},
		// April: one backup (no May either — another gap).
		{Name: "april", LastModified: mustParse(t, "2026-04-15T00:00:00Z")},
	}
	policy := Policy{RecentCount: 3, MonthlyCount: 12}

	keep := Decide(blobs, policy, now)

	for _, name := range []string{"recent-1", "recent-2", "recent-3", "july", "april"} {
		if !keep[name] {
			t.Errorf("expected %q to be kept, got not kept", name)
		}
	}
	if len(keep) != 5 {
		t.Errorf("expected 5 kept blobs (no phantom entries for June/May), got %d: %v", len(keep), keep)
	}
}

// TestDecide_MultipleInSameMonth covers "múltiplos backups no mesmo mês" —
// only the most recent one in a given month (outside the RecentCount top)
// must be kept.
func TestDecide_MultipleInSameMonth(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	blobs := []BlobInfo{
		{Name: "recent-1", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "recent-2", LastModified: mustParse(t, "2026-09-13T00:00:00Z")},
		{Name: "recent-3", LastModified: mustParse(t, "2026-09-12T00:00:00Z")},
		// Three May backups outside the top 3 — only the most recent (May 20) should survive.
		{Name: "may-20", LastModified: mustParse(t, "2026-05-20T00:00:00Z")},
		{Name: "may-10", LastModified: mustParse(t, "2026-05-10T00:00:00Z")},
		{Name: "may-01", LastModified: mustParse(t, "2026-05-01T00:00:00Z")},
	}
	policy := Policy{RecentCount: 3, MonthlyCount: 12}

	keep := Decide(blobs, policy, now)

	if !keep["may-20"] {
		t.Errorf("expected may-20 (most recent in May) to be kept")
	}
	if keep["may-10"] || keep["may-01"] {
		t.Errorf("expected only the most recent May blob to be kept, got may-10=%v may-01=%v", keep["may-10"], keep["may-01"])
	}
	if len(keep) != 4 {
		t.Errorf("expected 4 kept blobs (3 recent + 1 for May), got %d: %v", len(keep), keptNames(keep))
	}
}

// TestDecide_MonthlyBoundary covers blobs sitting exactly at the edge of the
// MonthlyCount-month window.
func TestDecide_MonthlyBoundary(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	policy := Policy{RecentCount: 0, MonthlyCount: 12}

	// Exactly 12 months back from now is 2025-09-14. A blob dated
	// 2025-09-20 (inside the window) must be kept; a blob dated
	// 2025-08-20 (13 months back) must not.
	blobs := []BlobInfo{
		{Name: "inside-window", LastModified: mustParse(t, "2025-09-20T00:00:00Z")},
		{Name: "outside-window", LastModified: mustParse(t, "2025-08-20T00:00:00Z")},
	}

	keep := Decide(blobs, policy, now)

	if !keep["inside-window"] {
		t.Errorf("expected inside-window blob (within last 12 months) to be kept")
	}
	if keep["outside-window"] {
		t.Errorf("expected outside-window blob (13 months back) to not be kept")
	}
}

// TestDecide_RecentCountZero covers a policy with RecentCount=0 (allowed as
// long as MonthlyCount>0, per RN-BACKUP-004 — never both zero).
func TestDecide_RecentCountZero(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	policy := Policy{RecentCount: 0, MonthlyCount: 12}

	blobs := []BlobInfo{
		{Name: "sept-1", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "sept-2", LastModified: mustParse(t, "2026-09-01T00:00:00Z")},
	}

	keep := Decide(blobs, policy, now)

	// With RecentCount=0, nothing is kept "unconditionally" — but the most
	// recent September blob still gets kept as that month's representative.
	if !keep["sept-1"] {
		t.Errorf("expected sept-1 (most recent in its month) to be kept even with RecentCount=0")
	}
	if keep["sept-2"] {
		t.Errorf("expected sept-2 to not be kept (superseded by sept-1 within the same month)")
	}
}

// TestDecide_MonthlyCountZero covers a policy with MonthlyCount=0 (allowed
// as long as RecentCount>0) — only the RecentCount most recent blobs survive,
// nothing else.
func TestDecide_MonthlyCountZero(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	policy := Policy{RecentCount: 2, MonthlyCount: 0}

	blobs := []BlobInfo{
		{Name: "recent-1", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "recent-2", LastModified: mustParse(t, "2026-09-13T00:00:00Z")},
		{Name: "older", LastModified: mustParse(t, "2026-08-01T00:00:00Z")},
	}

	keep := Decide(blobs, policy, now)

	if len(keep) != 2 || !keep["recent-1"] || !keep["recent-2"] {
		t.Errorf("expected exactly the 2 most recent blobs kept, got %v", keptNames(keep))
	}
}

// fakeDelete records calls and can be configured to fail after N successful
// calls, to exercise Sweep's stop-on-error behavior.
type fakeDelete struct {
	calls   []string
	failAt  int // -1 means never fail
	callErr error
}

func (f *fakeDelete) delete(ctx context.Context, blobName string) error {
	if f.failAt >= 0 && len(f.calls) == f.failAt {
		f.calls = append(f.calls, blobName) // still record the attempted call
		return f.callErr
	}
	f.calls = append(f.calls, blobName)
	return nil
}

func TestSweep_DryRunNeverDeletes(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	policy := Policy{RecentCount: 1, MonthlyCount: 0}
	blobs := []BlobInfo{
		{Name: "keep-me", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "delete-me-1", LastModified: mustParse(t, "2026-08-01T00:00:00Z")},
		{Name: "delete-me-2", LastModified: mustParse(t, "2026-07-01T00:00:00Z")},
	}

	fd := &fakeDelete{failAt: -1}
	affected, err := Sweep(context.Background(), blobs, policy, now, true, fd.delete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fd.calls) != 0 {
		t.Fatalf("dryRun=true must never call DeleteFunc, but it was called with: %v", fd.calls)
	}

	wantAffected := map[string]bool{"delete-me-1": true, "delete-me-2": true}
	if len(affected) != len(wantAffected) {
		t.Fatalf("expected %d affected blobs, got %d: %v", len(wantAffected), len(affected), affected)
	}
	for _, name := range affected {
		if !wantAffected[name] {
			t.Errorf("unexpected blob in affected list: %q", name)
		}
	}
}

func TestSweep_RealRunDeletesEachNotKept(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	policy := Policy{RecentCount: 1, MonthlyCount: 0}
	blobs := []BlobInfo{
		{Name: "keep-me", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "delete-me-1", LastModified: mustParse(t, "2026-08-01T00:00:00Z")},
		{Name: "delete-me-2", LastModified: mustParse(t, "2026-07-01T00:00:00Z")},
	}

	fd := &fakeDelete{failAt: -1}
	affected, err := Sweep(context.Background(), blobs, policy, now, false, fd.delete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fd.calls) != 2 {
		t.Fatalf("expected DeleteFunc called exactly twice, got %d calls: %v", len(fd.calls), fd.calls)
	}
	for _, name := range fd.calls {
		if name == "keep-me" {
			t.Errorf("DeleteFunc must never be called for a kept blob, but was called for %q", name)
		}
	}
	if len(affected) != 2 {
		t.Errorf("expected 2 affected blobs, got %d: %v", len(affected), affected)
	}
}

func TestSweep_StopsOnDeleteError(t *testing.T) {
	now := mustParse(t, "2026-09-14T12:00:00Z")
	policy := Policy{RecentCount: 0, MonthlyCount: 0}
	blobs := []BlobInfo{
		{Name: "a", LastModified: mustParse(t, "2026-09-14T00:00:00Z")},
		{Name: "b", LastModified: mustParse(t, "2026-09-13T00:00:00Z")},
		{Name: "c", LastModified: mustParse(t, "2026-09-12T00:00:00Z")},
	}

	wantErr := errors.New("azure: delete failed")
	fd := &fakeDelete{failAt: 1, callErr: wantErr} // fails on the 2nd delete call

	affected, err := Sweep(context.Background(), blobs, policy, now, false, fd.delete)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected Sweep to return the delete error, got: %v", err)
	}
	// Only the first successful deletion should be reflected in affected;
	// Sweep must stop immediately after the failing call, not continue to
	// the remaining blobs.
	if len(affected) != 1 {
		t.Fatalf("expected exactly 1 affected blob before the error, got %d: %v", len(affected), affected)
	}
	if len(fd.calls) != 2 {
		t.Fatalf("expected exactly 2 DeleteFunc calls (1 success + 1 failure), got %d: %v", len(fd.calls), fd.calls)
	}
}

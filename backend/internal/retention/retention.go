// Package retention implements the GFS-style (Grandfather-Father-Son)
// retention algorithm described in RN-BACKUP-003: keep the N most recent
// backups unconditionally, plus one backup per calendar month for the last
// M months. Both Decide and Sweep are pure with respect to I/O — Sweep only
// performs I/O through the DeleteFunc injected by the caller — so the
// algorithm can be exercised by unit tests without a real Azure Blob
// Storage account.
package retention

import (
	"context"
	"sort"
	"time"
)

// BlobInfo is the minimal information the retention algorithm needs about a
// stored backup blob.
type BlobInfo struct {
	Name         string
	LastModified time.Time
}

// Policy mirrors domain.RetentionPolicy's counts (RecentCount, MonthlyCount).
type Policy struct {
	RecentCount  int
	MonthlyCount int
}

// Decide returns the set of blob names that should be kept, given the
// policy and the current time. It is a pure function: no I/O, no mutation
// of the input slice.
//
// Algorithm (RN-BACKUP-003):
//  1. Sort blobs by LastModified, descending (most recent first).
//  2. Keep the first p.RecentCount blobs unconditionally.
//  3. For the remaining blobs, walk them in the same descending order and
//     keep the first one encountered for each "YYYY-MM" month key, as long
//     as that month falls within the last p.MonthlyCount months counting
//     back from now. Because the list is already sorted descending, the
//     first blob seen for a given month is always the most recent one in
//     that month.
//  4. Everything else is not kept.
func Decide(blobs []BlobInfo, p Policy, now time.Time) map[string]bool {
	sorted := make([]BlobInfo, len(blobs))
	copy(sorted, blobs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].LastModified.After(sorted[j].LastModified)
	})

	keep := make(map[string]bool, len(sorted))
	monthsSeen := make(map[string]bool)
	cutoff := now.AddDate(0, -p.MonthlyCount, 0)

	for i, b := range sorted {
		if i < p.RecentCount {
			keep[b.Name] = true
			continue
		}
		if p.MonthlyCount <= 0 {
			continue
		}
		if b.LastModified.Before(cutoff) {
			continue
		}
		monthKey := b.LastModified.UTC().Format("2006-01")
		if !monthsSeen[monthKey] {
			monthsSeen[monthKey] = true
			keep[b.Name] = true
		}
	}
	return keep
}

// DeleteFunc deletes a single blob by name. Injected so Sweep stays
// testable without a real Azure client — the azureblob package supplies a
// DeleteFunc backed by azureblob.DeleteBlob in production.
type DeleteFunc func(ctx context.Context, blobName string) error

// Sweep computes Decide() and, unless dryRun is true, deletes every blob
// that Decide() did not keep. It returns the list of blob names that were
// (dryRun=false) or would be (dryRun=true) deleted, in no particular order.
//
// If del returns an error partway through, Sweep stops immediately and
// returns the error together with the list of blobs successfully deleted so
// far — the caller (the backup-now handler) is responsible for recording an
// audit entry (retention_deletions) for each name in that partial list
// before surfacing the error, so the audit trail never silently loses a
// real deletion.
func Sweep(ctx context.Context, blobs []BlobInfo, p Policy, now time.Time, dryRun bool, del DeleteFunc) ([]string, error) {
	keep := Decide(blobs, p, now)
	var affected []string
	for _, b := range blobs {
		if keep[b.Name] {
			continue
		}
		if dryRun {
			affected = append(affected, b.Name)
			continue
		}
		if err := del(ctx, b.Name); err != nil {
			return affected, err
		}
		affected = append(affected, b.Name)
	}
	return affected, nil
}

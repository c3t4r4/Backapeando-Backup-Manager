package backupcore

import (
	"fmt"
	"strings"
	"time"

	"backapeando-backup-manager/internal/domain"
)

// Slugify converts a name into a safe, single-path-segment folder/file
// fragment for blob storage: lowercased, non-alphanumeric runs collapsed to
// a single hyphen, leading/trailing hyphens trimmed. This is a security
// boundary — the result is concatenated into blob names, so it must never
// produce "..", a leading "/", or an embedded "/".
func Slugify(name string) string {
	var b strings.Builder
	lastWasHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasHyphen = false
		default:
			if !lastWasHyphen && b.Len() > 0 {
				b.WriteRune('-')
				lastWasHyphen = true
			}
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		slug = "server"
	}
	return slug
}

// StoragePrefix returns the immutable blob folder for a server (no trailing
// slash). Prefer BlobPrefix when set (RN-BACKUP-034); fall back to
// Slugify(Name) for rows not yet backfilled.
func StoragePrefix(s domain.Server) string {
	if strings.TrimSpace(s.BlobPrefix) != "" {
		return s.BlobPrefix
	}
	return Slugify(s.Name)
}

// FormatBackupBlobName builds the canonical backup object name:
// `{storagePrefix}/{slug(dbName)}_{UTCcompact}.dump`.
// storagePrefix must not include a trailing slash.
func FormatBackupBlobName(storagePrefix, dbName string, t time.Time) string {
	return fmt.Sprintf("%s/%s_%s.dump",
		storagePrefix,
		Slugify(dbName),
		t.UTC().Format("20060102T150405Z"))
}

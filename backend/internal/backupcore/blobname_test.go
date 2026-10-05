package backupcore

import (
	"testing"
	"time"

	"backapeando-backup-manager/internal/domain"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"My Server", "my-server"},
		{"my-server", "my-server"},
		{"  Hello!!World  ", "hello-world"},
		{"...", "server"},
		{"", "server"},
		{"Foo/Bar", "foo-bar"},
		{"a..b", "a-b"},
	}
	for _, tc := range cases {
		got := Slugify(tc.in)
		if got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if stringsContainsUnsafe(got) {
			t.Errorf("Slugify(%q) = %q is not path-safe", tc.in, got)
		}
	}
}

func stringsContainsUnsafe(slug string) bool {
	if slug == "" || slug[0] == '.' {
		return true
	}
	for _, r := range slug {
		if r == '/' || r == '\\' {
			return true
		}
	}
	return false
}

func TestStoragePrefix(t *testing.T) {
	t.Run("uses BlobPrefix when set", func(t *testing.T) {
		s := domain.Server{Name: "New Name", BlobPrefix: "old-name"}
		if got := StoragePrefix(s); got != "old-name" {
			t.Fatalf("StoragePrefix = %q, want old-name", got)
		}
	})
	t.Run("falls back to Slugify(Name)", func(t *testing.T) {
		s := domain.Server{Name: "My Server", BlobPrefix: ""}
		if got := StoragePrefix(s); got != "my-server" {
			t.Fatalf("StoragePrefix = %q, want my-server", got)
		}
	})
}

func TestFormatBackupBlobName(t *testing.T) {
	ts := time.Date(2026, 9, 14, 15, 4, 5, 0, time.FixedZone("BRT", -3*3600))
	got := FormatBackupBlobName("my-server", "Prod_DB", ts)
	want := "my-server/prod-db_20260914T180405Z.dump"
	if got != want {
		t.Fatalf("FormatBackupBlobName = %q, want %q", got, want)
	}
}

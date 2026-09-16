package config

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// setRequiredEnv sets the env vars Load() always requires regardless of
// what this test is actually exercising (DATABASE_URL, a valid
// MASTER_ENCRYPTION_KEY), so each test only has to vary the var it cares
// about.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	key := make([]byte, 32)
	t.Setenv("MASTER_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
}

func TestLoad_SecureCookiesRequired_FailsWhenUnset(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when SECURE_COOKIES is unset, got nil")
	}
	if !strings.Contains(err.Error(), "SECURE_COOKIES") {
		t.Errorf("expected error to mention SECURE_COOKIES, got: %v", err)
	}
}

func TestLoad_SecureCookiesRequired_FailsOnInvalidValue(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "yes")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when SECURE_COOKIES is not exactly true/false, got nil")
	}
}

func TestLoad_SecureCookiesTrue(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if !cfg.SecureCookies {
		t.Error("expected SecureCookies=true")
	}
}

func TestLoad_SecureCookiesFalse(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.SecureCookies {
		t.Error("expected SecureCookies=false")
	}
}

func TestLoad_SecureCookiesCaseInsensitive(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "TRUE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if !cfg.SecureCookies {
		t.Error("expected SecureCookies=true for \"TRUE\"")
	}
}

func TestLoad_CORSAllowedOriginDefaultsToEmpty(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("CORS_ALLOWED_ORIGIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.CORSAllowedOrigin != "" {
		t.Errorf("expected empty CORSAllowedOrigin by default, got %q", cfg.CORSAllowedOrigin)
	}
}

func TestLoad_CORSAllowedOriginRespectsExplicitValue(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://app.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.CORSAllowedOrigin != "https://app.example.com" {
		t.Errorf("expected CORSAllowedOrigin to be set, got %q", cfg.CORSAllowedOrigin)
	}
}

func TestLoad_CORSAllowedOriginRejectsWildcard(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("CORS_ALLOWED_ORIGIN", "*")

	_, err := Load()
	if err == nil {
		t.Fatal(`expected an error when CORS_ALLOWED_ORIGIN is "*", got nil`)
	}
	if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGIN") {
		t.Errorf("expected error to mention CORS_ALLOWED_ORIGIN, got: %v", err)
	}
}

// TestLoad_DatabaseURLFile is a regression test for a bug where
// DATABASE_URL_FILE/MASTER_ENCRYPTION_KEY_FILE (the Docker Swarm secrets
// convention already used by docker-compose.prod.yml) were never actually
// resolved — Load() only read the plain env var, so a real Swarm deployment
// mounting secrets at /run/secrets/* would fail to boot with "DATABASE_URL
// is required" despite the secret being present on disk.
func TestLoad_DatabaseURLFile(t *testing.T) {
	t.Setenv("SECURE_COOKIES", "true")
	key := make([]byte, 32)
	t.Setenv("MASTER_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))

	dsnFile := t.TempDir() + "/database_url"
	dsn := "postgres://user:pass@localhost/db"
	if err := os.WriteFile(dsnFile, []byte(dsn+"\n"), 0o600); err != nil {
		t.Fatalf("write dsn file: %v", err)
	}
	t.Setenv("DATABASE_URL_FILE", dsnFile)
	t.Setenv("DATABASE_URL", "") // must be ignored in favor of the _FILE variant

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.DSN != dsn {
		t.Errorf("expected DSN %q (trimmed from file), got %q", dsn, cfg.DSN)
	}
}

func TestLoad_MasterEncryptionKeyFile(t *testing.T) {
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(key)

	keyFile := t.TempDir() + "/master_key"
	if err := os.WriteFile(keyFile, []byte(encoded), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	t.Setenv("MASTER_ENCRYPTION_KEY_FILE", keyFile)
	t.Setenv("MASTER_ENCRYPTION_KEY", "") // must be ignored in favor of the _FILE variant

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if string(cfg.MasterKey) != string(key) {
		t.Errorf("expected MasterKey read from file to match, got mismatch")
	}
}

func TestLoad_FileVariantMissingFileFailsFast(t *testing.T) {
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("MASTER_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("DATABASE_URL_FILE", "/nonexistent/path/database_url")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL_FILE points to a nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL_FILE") {
		t.Errorf("expected error to mention DATABASE_URL_FILE, got: %v", err)
	}
}

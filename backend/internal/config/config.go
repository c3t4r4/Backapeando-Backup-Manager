package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all environment-derived settings for the API process.
// It fails fast at boot if anything required is missing or malformed,
// rather than surfacing a confusing error deep inside a request handler.
type Config struct {
	HTTPAddr string
	DSN      string

	MasterKey         []byte // 32 bytes, AES-256-GCM
	MasterKeyPrevious []byte // optional, for key rotation; nil if unset

	SessionCookieName  string
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	SecureCookies      bool

	// CORSAllowedOrigin, when non-empty, enables CORS response headers for
	// exactly this origin. Empty (default) means no CORS headers are sent
	// at all — the correct setting whenever the frontend and API are
	// served same-origin (via the Nginx/Traefik proxy in this project's
	// docker-compose setups).
	CORSAllowedOrigin string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          getEnvDefault("HTTP_ADDR", ":8081"),
		SessionCookieName: getEnvDefault("SESSION_COOKIE_NAME", "backapeando_backup_session"),
	}

	dsn, err := getEnvOrFile("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	if dsn == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	cfg.DSN = dsn

	masterKeyRaw, err := getEnvOrFile("MASTER_ENCRYPTION_KEY")
	if err != nil {
		return Config{}, err
	}
	masterKey, err := decodeKey(masterKeyRaw)
	if err != nil {
		return Config{}, fmt.Errorf("MASTER_ENCRYPTION_KEY: %w", err)
	}
	cfg.MasterKey = masterKey

	prevRaw, err := getEnvOrFile("MASTER_ENCRYPTION_KEY_PREVIOUS")
	if err != nil {
		return Config{}, err
	}
	if prevRaw != "" {
		prev, err := decodeKey(prevRaw)
		if err != nil {
			return Config{}, fmt.Errorf("MASTER_ENCRYPTION_KEY_PREVIOUS: %w", err)
		}
		cfg.MasterKeyPrevious = prev
	}

	idleTTL, err := getEnvDurationDefault("SESSION_IDLE_TTL", 12*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.SessionIdleTTL = idleTTL

	absTTL, err := getEnvDurationDefault("SESSION_ABSOLUTE_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.SessionAbsoluteTTL = absTTL

	secureCookies, err := getEnvRequiredBool("SECURE_COOKIES")
	if err != nil {
		return Config{}, err
	}
	cfg.SecureCookies = secureCookies

	corsOrigin := getEnvDefault("CORS_ALLOWED_ORIGIN", "")
	if corsOrigin == "*" {
		// Browsers already reject `Access-Control-Allow-Origin: *` combined
		// with `Access-Control-Allow-Credentials: true` client-side, so
		// this would be a silent no-op rather than an exploitable
		// misconfiguration — but fail fast anyway, consistent with this
		// file's philosophy: a wrong value here should be caught at boot,
		// not discovered later as "CORS mysteriously doesn't work".
		return Config{}, fmt.Errorf(`CORS_ALLOWED_ORIGIN must not be "*" — this API always requires credentials (session + CSRF cookies), which browsers refuse to combine with a wildcard origin; set exactly one origin or leave empty`)
	}
	cfg.CORSAllowedOrigin = corsOrigin

	return cfg, nil
}

func decodeKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("must be set (base64-encoded 32 bytes, e.g. `openssl rand -base64 32`)")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("must be valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("must decode to exactly 32 bytes for AES-256, got %d", len(key))
	}
	return key, nil
}

// getEnvOrFile implements the standard Docker/Swarm secrets convention: if
// <KEY>_FILE is set, its value is a path to a file whose trimmed contents
// become the setting (this is how Docker Swarm secrets are meant to be
// consumed — mounted read-only at /run/secrets/<name>, never placed in the
// process environment where `docker service inspect`/`docker inspect` would
// expose them in plaintext). Falls back to the plain <KEY> env var when no
// _FILE variant is set, so this is a no-op for every non-Swarm deployment
// (local dev, docker-compose.yml) that just sets the var directly.
func getEnvOrFile(key string) (string, error) {
	if filePath := os.Getenv(key + "_FILE"); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("%s_FILE: read %s: %w", key, filePath, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return os.Getenv(key), nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvRequiredBool fails fast if the env var is unset or is anything
// other than exactly "true"/"false" (case-insensitive). This replaces a
// prior implicit default (unset == secure) that silently enabled Secure
// cookies over plain HTTP in any deployment that forgot to set this var —
// every real deployment of this project already sets it explicitly
// (docker-compose.yml, docker-compose.prod.yml, .env.example), so this is
// not expected to break anything, only to stop a future accidental one.
func getEnvRequiredBool(key string) (bool, error) {
	raw := os.Getenv(key)
	switch strings.ToLower(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s is required and must be exactly \"true\" or \"false\", got %q", key, raw)
	}
}

func getEnvDurationDefault(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer number of seconds: %w", key, err)
	}
	return time.Duration(seconds) * time.Second, nil
}

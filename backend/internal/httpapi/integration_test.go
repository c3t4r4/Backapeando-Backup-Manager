package httpapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"testing"
	"time"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/cpf"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/httpapi"
	"backapeando-backup-manager/internal/repository"
)

// TestPhase1EndToEnd exercises the whole Phase 1 stack against a real
// Postgres: migrations, admin bootstrap, session login, CSRF-protected
// CRUD for servers/storage-targets/retention-policies, and logout.
//
// It requires a real database and is skipped unless DATABASE_URL is set
// (see backend/README.md for how to point it at a throwaway container).
func TestPhase1EndToEnd(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	pool, err := repository.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	repos := repository.New(pool)

	// Unique per run so repeated local runs against a leftover (non-CI,
	// not-recreated) database don't collide on the admin_users email/cpf
	// uniqueness constraints; cleaned up at the end regardless of outcome.
	adminEmail := fmt.Sprintf("admin-%d@backapeando.local", time.Now().UnixNano())
	adminCPF, err := cpf.GenerateValidForTests(fmt.Sprintf("%09d", time.Now().UnixNano()%1_000_000_000))
	if err != nil {
		t.Fatalf("generate test cpf: %v", err)
	}

	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	sealer, err := crypto.NewSealer(masterKey, nil)
	if err != nil {
		t.Fatalf("new sealer: %v", err)
	}

	sessions := &auth.SessionManager{
		Sessions:    repos.AdminSessions,
		CookieName:  "test_session",
		IdleTTL:     time.Hour,
		AbsoluteTTL: 24 * time.Hour,
		Secure:      false,
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Repos:         repos,
		Sessions:      sessions,
		Sealer:        sealer,
		RateLimiter:   auth.NewLoginRateLimiter(100, time.Minute),
		SecureCookies: false,
	})

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}

	// --- bootstrap an admin directly via the repository, mirroring what
	// the `create-admin` CLI subcommand does (that command needs a real
	// TTY for its password prompt, so it's exercised here at the
	// repository/hashing layer instead). ---
	hash, err := auth.HashPassword("supersecretpassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	createdAdmin, err := repos.AdminUsers.Create(ctx, adminEmail, adminCPF, hash, "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() {
		// admin_sessions cascades from admin_users. servers/storage_targets
		// don't reference admin_users, so they're deleted explicitly below
		// once their IDs are known, keeping repeated local runs collision-free.
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_users WHERE id = $1`, createdAdmin.ID)
	})

	// --- health check is public ---
	resp, err := client.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}

	// --- CRUD without auth must be rejected ---
	resp, err = client.Get(server.URL + "/api/servers")
	if err != nil {
		t.Fatalf("unauthenticated list: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want 401", resp.StatusCode)
	}

	// --- login ---
	loginBody, _ := json.Marshal(map[string]string{
		"email": adminEmail, "password": "supersecretpassword123",
	})
	resp, err = client.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}

	csrfToken := cookieValue(t, jar, server.URL, "backapeando_backup_csrf")
	if csrfToken == "" {
		t.Fatal("expected csrf cookie to be set after login")
	}

	// --- wrong password must fail and must not reveal user existence ---
	wrongLoginBody, _ := json.Marshal(map[string]string{
		"email": adminEmail, "password": "wrong-password",
	})
	resp, err = client.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(wrongLoginBody))
	if err != nil {
		t.Fatalf("wrong login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong login status = %d, want 401", resp.StatusCode)
	}

	// --- create storage target (create-then-read; SAS token must never come back) ---
	targetBody, _ := json.Marshal(map[string]any{
		"name": "homolog", "type": "azure", "accountName": "backapeandostorage", "containerName": "backups",
		"sasToken": "sv=2022-11-02&ss=b&srt=co&sp=rwdl&se=2030-01-01T00:00:00Z&sig=fake",
	})
	req := newRequest(t, http.MethodPost, server.URL+"/api/storage-targets", targetBody, csrfToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}
	var target map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		t.Fatalf("decode storage target response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create storage target status = %d, want 201, body=%v", resp.StatusCode, target)
	}
	if _, leaked := target["sasToken"]; leaked {
		t.Fatal("SAS token must never be present in the API response")
	}
	storageTargetID, _ := target["id"].(string)
	if storageTargetID == "" {
		t.Fatal("expected storage target id in response")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM storage_targets WHERE id = $1`, storageTargetID)
	})

	// --- the SAS token must actually be decryptable using the ID the API
	// returned (regression check: Create used to encrypt against a
	// throwaway UUID that was never persisted as the row's real ID,
	// making the ciphertext permanently undecryptable) ---
	storedTarget, err := repos.StorageTargets.Get(ctx, storageTargetID)
	if err != nil {
		t.Fatalf("get storage target from repository: %v", err)
	}
	decrypted, err := sealer.Decrypt(storageTargetID, "sas_token", storedTarget.AzureSASTokenEncrypted)
	if err != nil {
		t.Fatalf("decrypt sas token with the API-returned id: %v", err)
	}
	if decrypted != "sv=2022-11-02&ss=b&srt=co&sp=rwdl&se=2030-01-01T00:00:00Z&sig=fake" {
		t.Fatalf("decrypted sas token = %q, want the original value", decrypted)
	}

	// --- create without CSRF token must be rejected ---
	req = newRequest(t, http.MethodPost, server.URL+"/api/storage-targets", targetBody, "")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("create without csrf: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("create without csrf status = %d, want 403", resp.StatusCode)
	}

	// --- create server referencing that storage target ---
	serverBody, _ := json.Marshal(map[string]any{
		"name": "prod", "host": "10.10.10.10", "port": 22, "sshUser": "ubuntu",
		"containerName": "postgres_postgres.1", "dbName": "backapeando_int", "dbUser": "backapeandoprod",
		"storageTargetId": storageTargetID, "cronExpression": "0 3 * * *",
	})
	req = newRequest(t, http.MethodPost, server.URL+"/api/servers", serverBody, csrfToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	var createdServer map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&createdServer); err != nil {
		t.Fatalf("decode server response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create server status = %d, want 201, body=%v", resp.StatusCode, createdServer)
	}
	if createdServer["status"] != "pending_key" {
		t.Fatalf("new server status = %v, want pending_key", createdServer["status"])
	}
	serverID, _ := createdServer["id"].(string)
	if serverID == "" {
		t.Fatal("expected server id in response")
	}
	t.Cleanup(func() {
		// Runs before the storage_targets cleanup above (t.Cleanup is LIFO),
		// which matters: servers.storage_target_id has no ON DELETE CASCADE,
		// so the referencing server must go first.
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, serverID)
	})

	// --- retention policy defaults to the global 3/12 for a server with no override ---
	resp, err = client.Get(server.URL + "/api/servers/" + serverID + "/retention-policy")
	if err != nil {
		t.Fatalf("get retention policy: %v", err)
	}
	var policy map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		t.Fatalf("decode retention policy: %v", err)
	}
	resp.Body.Close()
	if policy["recentCount"] != float64(3) || policy["monthlyCount"] != float64(12) {
		t.Fatalf("effective policy = %v, want recentCount=3 monthlyCount=12", policy)
	}

	// --- list servers reflects the created one ---
	resp, err = client.Get(server.URL + "/api/servers")
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode server list: %v", err)
	}
	resp.Body.Close()
	if len(list) != 1 {
		t.Fatalf("expected 1 server, got %d", len(list))
	}

	// --- logout, then confirm the session is dead server-side ---
	req = newRequest(t, http.MethodPost, server.URL+"/api/auth/logout", nil, csrfToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	resp.Body.Close()

	resp, err = client.Get(server.URL + "/api/servers")
	if err != nil {
		t.Fatalf("list after logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("list after logout status = %d, want 401", resp.StatusCode)
	}
}

func newRequest(t *testing.T, method, url string, body []byte, csrfToken string) *http.Request {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	return req
}

func cookieValue(t *testing.T, jar *cookiejar.Jar, rawURL, name string) string {
	t.Helper()
	u, err := neturl.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

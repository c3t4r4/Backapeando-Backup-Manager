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
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/cpf"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/httpapi"
	"backapeando-backup-manager/internal/repository"
)

// TestDashboardSummaryEndpoint exercises GET /api/dashboard/summary against
// a real Postgres: it must reject unauthenticated requests, and for an
// authenticated session it must return the KPI/attention-points shape the
// frontend's analytics dashboard depends on. It sets up its own admin/
// session independently of TestPhase1EndToEnd so it does not depend on
// that test's (unrelated, pre-existing) assertions succeeding.
//
// Requires a real database and is skipped unless DATABASE_URL is set.
func TestDashboardSummaryEndpoint(t *testing.T) {
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

	adminEmail := fmt.Sprintf("dashboard-admin-%d@backapeando.local", time.Now().UnixNano())
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

	hash, err := auth.HashPassword("supersecretpassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	createdAdmin, err := repos.AdminUsers.Create(ctx, adminEmail, adminCPF, hash, "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_users WHERE id = $1`, createdAdmin.ID)
	})

	// --- unauthenticated request must be rejected ---
	resp, err := client.Get(server.URL + "/api/dashboard/summary")
	if err != nil {
		t.Fatalf("unauthenticated summary: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated summary status = %d, want 401", resp.StatusCode)
	}

	// --- login ---
	loginBody := []byte(fmt.Sprintf(`{"email":%q,"password":"supersecretpassword123"}`, adminEmail))
	resp, err = client.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}

	// --- authenticated request must return the expected shape ---
	resp, err = client.Get(server.URL + "/api/dashboard/summary")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("summary status = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode summary response: %v", err)
	}

	servers, ok := body["servers"].(map[string]any)
	if !ok {
		t.Fatalf("summary.servers = %v, want an object", body["servers"])
	}
	if _, ok := servers["total"].(float64); !ok {
		t.Errorf("summary.servers.total = %v, want a number", servers["total"])
	}
	byStatus, ok := servers["byStatus"].(map[string]any)
	if !ok {
		t.Fatalf("summary.servers.byStatus = %v, want an object", servers["byStatus"])
	}
	for _, key := range []string{"pendingKey", "awaitingAuthorization", "ready", "disabled"} {
		if _, ok := byStatus[key].(float64); !ok {
			t.Errorf("summary.servers.byStatus.%s = %v, want a number", key, byStatus[key])
		}
	}

	runsWindow, ok := body["backupRunsLast30Days"].(map[string]any)
	if !ok {
		t.Fatalf("summary.backupRunsLast30Days = %v, want an object", body["backupRunsLast30Days"])
	}
	for _, key := range []string{"total", "success", "failed", "successRatePercent"} {
		if _, ok := runsWindow[key].(float64); !ok {
			t.Errorf("summary.backupRunsLast30Days.%s = %v, want a number", key, runsWindow[key])
		}
	}

	attention, ok := body["attentionPoints"].(map[string]any)
	if !ok {
		t.Fatalf("summary.attentionPoints = %v, want an object", body["attentionPoints"])
	}
	for _, key := range []string{"serversAwaitingAuthorization", "serversWithConnectionError", "recentFailures"} {
		if _, ok := attention[key].([]any); !ok {
			t.Errorf("summary.attentionPoints.%s = %v, want an array", key, attention[key])
		}
	}
}

// TestDashboardSummaryEndpoint_WithConnectionErrorCountNotTruncated is a
// regression test for a bug found by the Validator during review: the
// handler reused the same (attention-list-capped) slice to compute both the
// "servers with connection error" KPI and the attention-points list, so the
// KPI silently under-reported once more than dashboardAttentionListLimit
// (10) servers had a connection error. The KPI must reflect the real total;
// only the attention-points list is capped at 10.
func TestDashboardSummaryEndpoint_WithConnectionErrorCountNotTruncated(t *testing.T) {
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

	adminEmail := fmt.Sprintf("dashboard-admin-conn-err-%d@backapeando.local", time.Now().UnixNano())
	adminCPF, err := cpf.GenerateValidForTests(fmt.Sprintf("%09d", (time.Now().UnixNano()+1)%1_000_000_000))
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

	hash, err := auth.HashPassword("supersecretpassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	createdAdmin, err := repos.AdminUsers.Create(ctx, adminEmail, adminCPF, hash, "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_users WHERE id = $1`, createdAdmin.ID)
	})

	const serverCount = 12 // more than dashboardAttentionListLimit (10)
	errMsg := "ssh connection test failed"
	containerName := "testcontainer"
	for i := 0; i < serverCount; i++ {
		s, err := repos.Servers.Create(ctx, domain.Server{
			Name: fmt.Sprintf("conn-err-test-%d-%s", i, uuid.New().String()[:8]), Host: "example.com", Port: 22,
			SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
			ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
			CronExpression: "0 3 * * *",
		})
		if err != nil {
			t.Fatalf("create server %d: %v", i, err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, s.ID)
		})
		if err := repos.Servers.RecordTestConnectionResult(ctx, s.ID, domain.ServerStatusAwaitingAuthorization, false, nil, &errMsg); err != nil {
			t.Fatalf("RecordTestConnectionResult %d: %v", i, err)
		}
	}

	loginBody := []byte(fmt.Sprintf(`{"email":%q,"password":"supersecretpassword123"}`, adminEmail))
	resp, err := client.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}

	resp, err = client.Get(server.URL + "/api/dashboard/summary")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("summary status = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode summary response: %v", err)
	}

	servers, ok := body["servers"].(map[string]any)
	if !ok {
		t.Fatalf("summary.servers = %v, want an object", body["servers"])
	}
	withConnectionError, ok := servers["withConnectionError"].(float64)
	if !ok {
		t.Fatalf("summary.servers.withConnectionError = %v, want a number", servers["withConnectionError"])
	}
	if int(withConnectionError) < serverCount {
		t.Errorf("summary.servers.withConnectionError = %v, want at least %d (the real total, not capped at the attention-list limit)", withConnectionError, serverCount)
	}

	attention, ok := body["attentionPoints"].(map[string]any)
	if !ok {
		t.Fatalf("summary.attentionPoints = %v, want an object", body["attentionPoints"])
	}
	attentionList, ok := attention["serversWithConnectionError"].([]any)
	if !ok {
		t.Fatalf("summary.attentionPoints.serversWithConnectionError = %v, want an array", attention["serversWithConnectionError"])
	}
	if len(attentionList) > 10 {
		t.Errorf("summary.attentionPoints.serversWithConnectionError has %d items, want at most 10 (dashboardAttentionListLimit)", len(attentionList))
	}
}

// TestDashboardBackupStatsEndpoint exercises GET /api/dashboard/backup-stats:
// it must reject unauthenticated requests, always return the 30-daily/
// 12-monthly label shape even with no data, always include the synthetic
// "Sem destino" entry, and reflect a real successful run at the position
// corresponding to today/this month for its destination (RN-BACKUP-031).
func TestDashboardBackupStatsEndpoint(t *testing.T) {
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

	adminEmail := fmt.Sprintf("dashboard-admin-backup-stats-%d@backapeando.local", time.Now().UnixNano())
	adminCPF, err := cpf.GenerateValidForTests(fmt.Sprintf("%09d", (time.Now().UnixNano()+2)%1_000_000_000))
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

	hash, err := auth.HashPassword("supersecretpassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	createdAdmin, err := repos.AdminUsers.Create(ctx, adminEmail, adminCPF, hash, "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_users WHERE id = $1`, createdAdmin.ID)
	})

	// --- unauthenticated request must be rejected ---
	resp, err := client.Get(server.URL + "/api/dashboard/backup-stats")
	if err != nil {
		t.Fatalf("unauthenticated backup-stats: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated backup-stats status = %d, want 401", resp.StatusCode)
	}

	loginBody := []byte(fmt.Sprintf(`{"email":%q,"password":"supersecretpassword123"}`, adminEmail))
	resp, err = client.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}

	// --- with no backup data yet, the shape must still be fully populated ---
	resp, err = client.Get(server.URL + "/api/dashboard/backup-stats")
	if err != nil {
		t.Fatalf("backup-stats: %v", err)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		resp.Body.Close()
		t.Fatalf("decode backup-stats response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("backup-stats status = %d, want 200", resp.StatusCode)
	}

	destinations, ok := body["destinations"].([]any)
	if !ok {
		t.Fatalf("backup-stats.destinations = %v, want an array", body["destinations"])
	}
	if len(destinations) == 0 {
		t.Fatalf("backup-stats.destinations is empty, want at least the synthetic \"Sem destino\" entry")
	}
	hasNilDestination := false
	for _, d := range destinations {
		dm, ok := d.(map[string]any)
		if !ok {
			t.Fatalf("destinations entry = %v, want an object", d)
		}
		if dm["storageTargetId"] == nil {
			hasNilDestination = true
			if dm["name"] != "Sem destino" {
				t.Errorf(`nil-destination entry name = %v, want "Sem destino"`, dm["name"])
			}
		}
	}
	if !hasNilDestination {
		t.Errorf("backup-stats.destinations does not include the synthetic nil destination entry")
	}

	daily, ok := body["dailyLast30Days"].(map[string]any)
	if !ok {
		t.Fatalf("backup-stats.dailyLast30Days = %v, want an object", body["dailyLast30Days"])
	}
	dailyLabels, ok := daily["labels"].([]any)
	if !ok || len(dailyLabels) != 30 {
		t.Errorf("backup-stats.dailyLast30Days.labels has %d items, want exactly 30", len(dailyLabels))
	}

	monthly, ok := body["monthlyThisYear"].(map[string]any)
	if !ok {
		t.Fatalf("backup-stats.monthlyThisYear = %v, want an object", body["monthlyThisYear"])
	}
	monthlyLabels, ok := monthly["labels"].([]any)
	if !ok || len(monthlyLabels) != 12 {
		t.Errorf("backup-stats.monthlyThisYear.labels has %d items, want exactly 12 (Jan-Dec)", len(monthlyLabels))
	}

	for _, period := range []map[string]any{daily, monthly} {
		labels, _ := period["labels"].([]any)
		for _, seriesKey := range []string{"countSeries", "bytesSeries"} {
			series, ok := period[seriesKey].([]any)
			if !ok {
				t.Fatalf("%s = %v, want an array", seriesKey, period[seriesKey])
			}
			if len(series) != len(destinations) {
				t.Errorf("len(%s) = %d, want %d (one per destination)", seriesKey, len(series), len(destinations))
			}
			for _, s := range series {
				sm, ok := s.(map[string]any)
				if !ok {
					t.Fatalf("%s entry = %v, want an object", seriesKey, s)
				}
				data, ok := sm["data"].([]any)
				if !ok || len(data) != len(labels) {
					t.Errorf("%s entry data has %d items, want %d (same as labels)", seriesKey, len(data), len(labels))
				}
			}
		}
	}

	// --- a real successful run must show up at today's/this month's position ---
	rootPath := "/tmp/backapeando-test"
	target, err := repos.StorageTargets.Create(ctx, domain.StorageTarget{
		ID:   uuid.New().String(),
		Name: "dashboard-endpoint-target-" + uuid.New().String()[:8],
		Type: domain.StorageTargetTypeFilesystem, FSRootPath: &rootPath,
	})
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM storage_targets WHERE id = $1`, target.ID)
	})

	containerName := "testcontainer"
	backupServer, err := repos.Servers.Create(ctx, domain.Server{
		Name: "dashboard-endpoint-server-" + uuid.New().String()[:8], Host: "example.com", Port: 22,
		SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
		ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
		CronExpression: "0 3 * * *", StorageTargetID: &target.ID,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, backupServer.ID)
	})

	run, err := repos.BackupRuns.Create(ctx, backupServer.ID)
	if err != nil {
		t.Fatalf("create backup run: %v", err)
	}
	const knownBytes = 123456
	if err := repos.BackupRuns.MarkSuccess(ctx, run.ID, "backup.dump", knownBytes, 10, 10); err != nil {
		t.Fatalf("mark success: %v", err)
	}

	resp, err = client.Get(server.URL + "/api/dashboard/backup-stats")
	if err != nil {
		t.Fatalf("backup-stats (with data): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("backup-stats (with data) status = %d, want 200", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode backup-stats (with data) response: %v", err)
	}

	daily, ok = body["dailyLast30Days"].(map[string]any)
	if !ok {
		t.Fatalf("backup-stats.dailyLast30Days = %v, want an object", body["dailyLast30Days"])
	}
	dailyLabels, _ = daily["labels"].([]any)
	todayLabel := time.Now().UTC().Format("2006-01-02")
	todayPos := -1
	for i, l := range dailyLabels {
		if l == todayLabel {
			todayPos = i
			break
		}
	}
	if todayPos == -1 {
		t.Fatalf("today's label %q not found in dailyLast30Days.labels", todayLabel)
	}

	bytesSeries, ok := daily["bytesSeries"].([]any)
	if !ok {
		t.Fatalf("backup-stats.dailyLast30Days.bytesSeries = %v, want an array", daily["bytesSeries"])
	}
	found := false
	for _, s := range bytesSeries {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if sm["storageTargetId"] != target.ID {
			continue
		}
		found = true
		data, _ := sm["data"].([]any)
		if todayPos >= len(data) || data[todayPos].(float64) < float64(knownBytes) {
			t.Errorf("bytesSeries[%s].data[%d] = %v, want at least %d", target.ID, todayPos, data[todayPos], knownBytes)
		}
	}
	if !found {
		t.Errorf("bytesSeries does not include an entry for storage target %s", target.ID)
	}
}

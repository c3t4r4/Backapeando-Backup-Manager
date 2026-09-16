package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backapeando-backup-manager/internal/cpf"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/httpapi/middleware"
	"backapeando-backup-manager/internal/repository"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateAdminUserRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     createAdminUserRequest
		wantErr string
	}{
		{name: "email required", req: createAdminUserRequest{Email: "  ", Password: "123456789012"}, wantErr: "email is required"},
		{name: "password too short", req: createAdminUserRequest{Email: "a@example.com", Password: "short"}, wantErr: "password must be at least 12 characters"},
		{name: "password exactly 12 chars is valid", req: createAdminUserRequest{Email: "a@example.com", Password: "123456789012"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want no error", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validate() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateAdminUserRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     updateAdminUserRequest
		wantErr string
	}{
		{name: "email required", req: updateAdminUserRequest{Email: "  "}, wantErr: "email is required"},
		{name: "empty password is valid (keep current)", req: updateAdminUserRequest{Email: "a@example.com", Password: ""}},
		{name: "short password rejected when provided", req: updateAdminUserRequest{Email: "a@example.com", Password: "short"}, wantErr: "password must be at least 12 characters"},
		{name: "password exactly 12 chars is valid", req: updateAdminUserRequest{Email: "a@example.com", Password: "123456789012"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want no error", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validate() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestWriteAdminUserConflictError(t *testing.T) {
	tests := []struct {
		name           string
		constraintName string
		wantBody       string
	}{
		{name: "email conflict", constraintName: "admin_users_email_key", wantBody: "email already registered"},
		{name: "cpf conflict", constraintName: "admin_users_cpf_unique", wantBody: "cpf already registered"},
		{name: "unknown constraint falls back to generic message", constraintName: "some_other_constraint", wantBody: "duplicate value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			pgErr := &pgconn.PgError{Code: "23505", ConstraintName: tt.constraintName}

			wrote := writeAdminUserConflictError(rr, pgErr)

			if !wrote {
				t.Fatal("writeAdminUserConflictError() = false, want true for a 23505 violation")
			}
			if rr.Code != 409 {
				t.Fatalf("status = %d, want 409", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", rr.Body.String(), tt.wantBody)
			}
		})
	}

	t.Run("non-conflict error is left unwritten", func(t *testing.T) {
		rr := httptest.NewRecorder()
		wrote := writeAdminUserConflictError(rr, errors.New("some unrelated db error"))
		if wrote {
			t.Fatal("writeAdminUserConflictError() = true, want false for a non-pgconn/non-23505 error")
		}
	})
}

// TestToAdminUserDTO_NeverIncludesPasswordHash guards RT-SEC-003
// (docs/RegrasNegocio.md): once written, a password must never be readable
// back through the API in any form.
func TestToAdminUserDTO_NeverIncludesPasswordHash(t *testing.T) {
	u := domain.AdminUser{
		ID:           "11111111-1111-1111-1111-111111111111",
		Email:        "admin@example.com",
		CPF:          "11144477735",
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$hash-that-must-never-leak",
		Role:         "admin",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	dto := toAdminUserDTO(u)
	body, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal DTO: %v", err)
	}
	if strings.Contains(string(body), "hash-that-must-never-leak") || strings.Contains(string(body), "passwordHash") || strings.Contains(string(body), "password_hash") {
		t.Fatalf("adminUserDTO JSON leaked the password hash: %s", body)
	}
}

// adminUserTestDB connects to a real Postgres for the handler integration
// tests below — AdminUserHandlers has no fake/mock substitute for
// AdminUserRepo in this codebase (see internal/repository's package doc on
// hand-written SQL for auditability). Requires DATABASE_URL; skipped
// otherwise, matching internal/repository/servers_test.go and
// storage_targets_test.go.
func adminUserTestDB(t *testing.T) *repository.Repositories {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return repository.New(pool)
}

func doCreateAdminUserRequest(t *testing.T, h *AdminUserHandlers, req createAdminUserRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq := httptest.NewRequest("POST", "/api/admin-users", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, httpReq)
	return rr
}

func doUpdateAdminUserRequest(t *testing.T, h *AdminUserHandlers, id string, req updateAdminUserRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq := httptest.NewRequest("PUT", "/api/admin-users/"+id, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	h.Update(rr, httpReq)
	return rr
}

// doDeleteAdminUserRequest drives Delete through the real http.Handler path
// with the given session (as middleware.RequireAuth would stash it in the
// request context after authenticating a real cookie).
func doDeleteAdminUserRequest(t *testing.T, h *AdminUserHandlers, id string, session domain.AdminSession) *httptest.ResponseRecorder {
	t.Helper()
	httpReq := httptest.NewRequest("DELETE", "/api/admin-users/"+id, nil)
	httpReq.SetPathValue("id", id)
	httpReq = httpReq.WithContext(middleware.WithSession(httpReq.Context(), session))
	rr := httptest.NewRecorder()
	h.Delete(rr, httpReq)
	return rr
}

func newTestCPF(t *testing.T, base string) string {
	t.Helper()
	c, err := cpf.GenerateValidForTests(base)
	if err != nil {
		t.Fatalf("generate test cpf: %v", err)
	}
	return c
}

// TestAdminUserHandlers_CRUD_HappyPath covers create -> list -> get ->
// update (email/cpf) -> update (password) -> delete, exercising every
// handler through the real http.Handler path against a live database.
func TestAdminUserHandlers_CRUD_HappyPath(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	email := "crud-happy-" + newTestCPF(t, "100000001") + "@example.com"
	createReq := createAdminUserRequest{Email: email, CPF: newTestCPF(t, "100000001"), Password: "correct-horse-battery"}

	createRR := doCreateAdminUserRequest(t, h, createReq)
	if createRR.Code != 201 {
		t.Fatalf("create status = %d, want 201, body=%s", createRR.Code, createRR.Body.String())
	}
	var created adminUserDTO
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.Email != email || created.Role != "admin" {
		t.Fatalf("created DTO = %+v, want email=%q role=admin", created, email)
	}
	t.Cleanup(func() { _ = repos.AdminUsers.Delete(context.Background(), created.ID) })

	// List must include the newly created admin.
	listReq := httptest.NewRequest("GET", "/api/admin-users", nil)
	listRR := httptest.NewRecorder()
	h.List(listRR, listReq)
	if listRR.Code != 200 {
		t.Fatalf("list status = %d, want 200", listRR.Code)
	}
	var listed []adminUserDTO
	if err := json.Unmarshal(listRR.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	found := false
	for _, u := range listed {
		if u.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list response does not include the created admin %q", created.ID)
	}

	// Get by id.
	getReq := httptest.NewRequest("GET", "/api/admin-users/"+created.ID, nil)
	getReq.SetPathValue("id", created.ID)
	getRR := httptest.NewRecorder()
	h.Get(getRR, getReq)
	if getRR.Code != 200 {
		t.Fatalf("get status = %d, want 200, body=%s", getRR.Code, getRR.Body.String())
	}

	// Update email/cpf, no password change.
	newEmail := "crud-happy-updated-" + newTestCPF(t, "100000002") + "@example.com"
	newCPF := newTestCPF(t, "100000002")
	updateRR := doUpdateAdminUserRequest(t, h, created.ID, updateAdminUserRequest{Email: newEmail, CPF: newCPF})
	if updateRR.Code != 200 {
		t.Fatalf("update status = %d, want 200, body=%s", updateRR.Code, updateRR.Body.String())
	}
	var updated adminUserDTO
	if err := json.Unmarshal(updateRR.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal update response: %v", err)
	}
	if updated.Email != newEmail || updated.CPF != newCPF {
		t.Fatalf("updated DTO = %+v, want email=%q cpf=%q", updated, newEmail, newCPF)
	}

	// Update with a new password: the stored hash must actually change.
	before, err := repos.AdminUsers.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get before password update: %v", err)
	}
	passwordUpdateRR := doUpdateAdminUserRequest(t, h, created.ID, updateAdminUserRequest{Email: newEmail, CPF: newCPF, Password: "another-strong-password-12"})
	if passwordUpdateRR.Code != 200 {
		t.Fatalf("password update status = %d, want 200, body=%s", passwordUpdateRR.Code, passwordUpdateRR.Body.String())
	}
	after, err := repos.AdminUsers.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get after password update: %v", err)
	}
	if after.PasswordHash == before.PasswordHash {
		t.Fatal("password update did not change the stored password hash")
	}
}

func TestAdminUserHandlers_Create_DuplicateEmailReturns409(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	email := "dup-email-" + newTestCPF(t, "200000001") + "@example.com"
	first := doCreateAdminUserRequest(t, h, createAdminUserRequest{Email: email, CPF: newTestCPF(t, "200000001"), Password: "correct-horse-battery"})
	if first.Code != 201 {
		t.Fatalf("first create status = %d, want 201, body=%s", first.Code, first.Body.String())
	}
	var created adminUserDTO
	_ = json.Unmarshal(first.Body.Bytes(), &created)
	t.Cleanup(func() { _ = repos.AdminUsers.Delete(context.Background(), created.ID) })

	second := doCreateAdminUserRequest(t, h, createAdminUserRequest{Email: email, CPF: newTestCPF(t, "200000002"), Password: "correct-horse-battery"})
	if second.Code != 409 {
		t.Fatalf("second create status = %d, want 409, body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "email already registered") {
		t.Fatalf("body = %q, want it to mention duplicate email", second.Body.String())
	}
}

func TestAdminUserHandlers_Create_DuplicateCPFReturns409(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	sharedCPF := newTestCPF(t, "200000003")
	first := doCreateAdminUserRequest(t, h, createAdminUserRequest{Email: "dup-cpf-a-" + sharedCPF + "@example.com", CPF: sharedCPF, Password: "correct-horse-battery"})
	if first.Code != 201 {
		t.Fatalf("first create status = %d, want 201, body=%s", first.Code, first.Body.String())
	}
	var created adminUserDTO
	_ = json.Unmarshal(first.Body.Bytes(), &created)
	t.Cleanup(func() { _ = repos.AdminUsers.Delete(context.Background(), created.ID) })

	second := doCreateAdminUserRequest(t, h, createAdminUserRequest{Email: "dup-cpf-b-" + sharedCPF + "@example.com", CPF: sharedCPF, Password: "correct-horse-battery"})
	if second.Code != 409 {
		t.Fatalf("second create status = %d, want 409, body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "cpf already registered") {
		t.Fatalf("body = %q, want it to mention duplicate cpf", second.Body.String())
	}
}

func TestAdminUserHandlers_Create_InvalidCPFReturns400(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	rr := doCreateAdminUserRequest(t, h, createAdminUserRequest{Email: "invalid-cpf@example.com", CPF: "00000000000", Password: "correct-horse-battery"})
	if rr.Code != 400 {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminUserHandlers_Create_ShortPasswordReturns400(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	rr := doCreateAdminUserRequest(t, h, createAdminUserRequest{Email: "short-pw@example.com", CPF: newTestCPF(t, "200000004"), Password: "tooshort"})
	if rr.Code != 400 {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminUserHandlers_Get_NotFoundReturns404(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	req := httptest.NewRequest("GET", "/api/admin-users/does-not-exist", nil)
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000000")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != 404 {
		t.Fatalf("status = %d, want 404, body=%s", rr.Code, rr.Body.String())
	}
}

// TestAdminUserHandlers_Delete_SelfBlocked covers RN-AUTH-002: an admin can
// never delete their own account, regardless of how many other admins
// exist. The self-check runs before the last-admin count check, so this is
// deterministic even against a shared database with an unknown number of
// pre-existing admins.
func TestAdminUserHandlers_Delete_SelfBlocked(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	createRR := doCreateAdminUserRequest(t, h, createAdminUserRequest{
		Email:    "self-delete-" + newTestCPF(t, "300000001") + "@example.com",
		CPF:      newTestCPF(t, "300000001"),
		Password: "correct-horse-battery",
	})
	if createRR.Code != 201 {
		t.Fatalf("create status = %d, want 201, body=%s", createRR.Code, createRR.Body.String())
	}
	var created adminUserDTO
	_ = json.Unmarshal(createRR.Body.Bytes(), &created)
	t.Cleanup(func() { _ = repos.AdminUsers.Delete(context.Background(), created.ID) })

	rr := doDeleteAdminUserRequest(t, h, created.ID, domain.AdminSession{UserID: created.ID})
	if rr.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cannot delete your own account") {
		t.Fatalf("body = %q, want it to mention self-deletion", rr.Body.String())
	}

	// The account must still exist after the blocked delete.
	if _, err := repos.AdminUsers.GetByID(context.Background(), created.ID); err != nil {
		t.Fatalf("admin should still exist after blocked self-delete, get error: %v", err)
	}
}

// TestAdminUserHandlers_Delete_NonLastAdminSucceeds covers the happy path
// of RN-AUTH-002/003: deleting an admin that is neither the caller nor the
// last one succeeds. Two fresh admins are created in this test, so the
// total count is guaranteed to be at least 2 at the time of the guard
// check, regardless of how many other admins already exist in the shared
// test database.
func TestAdminUserHandlers_Delete_NonLastAdminSucceeds(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	callerRR := doCreateAdminUserRequest(t, h, createAdminUserRequest{
		Email:    "caller-" + newTestCPF(t, "300000002") + "@example.com",
		CPF:      newTestCPF(t, "300000002"),
		Password: "correct-horse-battery",
	})
	if callerRR.Code != 201 {
		t.Fatalf("create caller status = %d, want 201, body=%s", callerRR.Code, callerRR.Body.String())
	}
	var caller adminUserDTO
	_ = json.Unmarshal(callerRR.Body.Bytes(), &caller)
	t.Cleanup(func() { _ = repos.AdminUsers.Delete(context.Background(), caller.ID) })

	targetRR := doCreateAdminUserRequest(t, h, createAdminUserRequest{
		Email:    "target-" + newTestCPF(t, "300000003") + "@example.com",
		CPF:      newTestCPF(t, "300000003"),
		Password: "correct-horse-battery",
	})
	if targetRR.Code != 201 {
		t.Fatalf("create target status = %d, want 201, body=%s", targetRR.Code, targetRR.Body.String())
	}
	var target adminUserDTO
	_ = json.Unmarshal(targetRR.Body.Bytes(), &target)

	rr := doDeleteAdminUserRequest(t, h, target.ID, domain.AdminSession{UserID: caller.ID})
	if rr.Code != 204 {
		t.Fatalf("status = %d, want 204, body=%s", rr.Code, rr.Body.String())
	}

	if _, err := repos.AdminUsers.GetByID(context.Background(), target.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("get after delete: err = %v, want ErrNotFound", err)
	}
}

// TestAdminUserHandlers_Delete_LastAdminBlocked covers RN-AUTH-003, but only
// when the shared test database happens to have exactly one admin at the
// moment the test runs — this codebase has no per-test schema reset, so the
// "last admin" scenario cannot be deterministically engineered without
// deleting other, possibly real, admin accounts. The check runs first and
// skips rather than faking a pass; see docs/Memoria.md for the same
// limitation noted for the repository-level guard.
func TestAdminUserHandlers_Delete_LastAdminBlocked(t *testing.T) {
	repos := adminUserTestDB(t)
	h := &AdminUserHandlers{Users: repos.AdminUsers}

	count, err := repos.AdminUsers.Count(context.Background())
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if count != 1 {
		t.Skipf("shared test database has %d admins, not exactly 1 — cannot deterministically exercise the last-admin guard without deleting real accounts", count)
	}

	lone, err := repos.AdminUsers.List(context.Background())
	if err != nil {
		t.Fatalf("list admins: %v", err)
	}
	if len(lone) != 1 {
		t.Fatalf("List() returned %d admins, want 1 to match Count()", len(lone))
	}

	rr := doDeleteAdminUserRequest(t, h, lone[0].ID, domain.AdminSession{UserID: "unrelated-session-user-id"})
	if rr.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cannot delete the last remaining admin") {
		t.Fatalf("body = %q, want it to mention the last-admin guard", rr.Body.String())
	}
}

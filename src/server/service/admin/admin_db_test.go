// SPDX-License-Identifier: MIT
// Coverage tests for AdminService methods that require a real database:
// Initialize, ValidateSetupToken, CreateAdmin, CreateAdminWithSetupToken,
// Authenticate, ChangePassword, GenerateInviteToken, CreateAPIToken,
// ValidateAPIToken, GetAdminCount, CreateAdminInvite, ValidateInviteToken,
// CreateAdminWithInvite, ListPendingInvites, RevokeInvite, CleanupExpiredInvites,
// GetAdmin, GetAPITokenInfo, RegenerateAPIToken, GenerateRecoveryKeys,
// ValidateRecoveryKey, GetRecoveryKeysStatus, CleanupExpiredSessions,
// CleanupExpiredTokens, GetTOTPSecret, GetTOTPBackupCodes, UseBackupCode,
// EnableTOTP, DisableTOTP.
package admin

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newAdminTestDB creates an in-memory SQLite DB with all tables required by
// AdminService. SetMaxOpenConns(1) keeps ":memory:" on a single connection.
func newAdminTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// admin_credentials — core admin table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS admin_credentials (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		totp_secret TEXT,
		totp_enabled INTEGER DEFAULT 0,
		totp_backup_codes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME,
		login_count INTEGER DEFAULT 0,
		is_primary INTEGER DEFAULT 0,
		invited_by INTEGER,
		invite_token TEXT,
		invite_expires DATETIME
	)`)
	if err != nil {
		t.Fatalf("create admin_credentials: %v", err)
	}

	// setup_tokens — includes username/created_by used by CreateAdminInvite
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS setup_tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token TEXT UNIQUE NOT NULL,
		purpose TEXT NOT NULL,
		username TEXT,
		created_by INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		used_at DATETIME,
		used_by TEXT
	)`)
	if err != nil {
		t.Fatalf("create setup_tokens: %v", err)
	}

	// api_tokens
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS api_tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		token_hash TEXT UNIQUE NOT NULL,
		token_prefix TEXT NOT NULL,
		permissions TEXT DEFAULT '*',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		last_used DATETIME,
		use_count INTEGER DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create api_tokens: %v", err)
	}

	// recovery_keys
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS recovery_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id INTEGER NOT NULL,
		key_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		used_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("create recovery_keys: %v", err)
	}

	// admin_sessions — used by CleanupExpiredSessions
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS admin_sessions (
		id TEXT PRIMARY KEY,
		admin_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create admin_sessions: %v", err)
	}

	return db
}

// newAdminSvc creates an AdminService backed by an in-memory test DB.
func newAdminSvc(t *testing.T) *AdminService {
	t.Helper()
	return NewAdminService(newAdminTestDB(t))
}

// mustCreateAdmin creates a primary admin and fails the test on error.
func mustCreateAdmin(t *testing.T, svc *AdminService, username, password string) *AdminUser {
	t.Helper()
	admin, err := svc.CreateAdmin(username, password, true)
	if err != nil {
		t.Fatalf("CreateAdmin(%q): %v", username, err)
	}
	return admin
}

// ── Initialize ────────────────────────────────────────────────────────────────

// TestAdminService_Initialize_FirstRun verifies that Initialize marks first run
// and generates a setup token when no admins exist.
func TestAdminService_Initialize_FirstRun(t *testing.T) {
	svc := newAdminSvc(t)
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !svc.IsFirstRun() {
		t.Error("IsFirstRun() = false on empty DB, want true")
	}
	if svc.GetSetupToken() == "" {
		t.Error("GetSetupToken() empty on first run")
	}
}

// TestAdminService_Initialize_NotFirstRunAfterAdmin verifies that Initialize
// sets isFirstRun=false when at least one admin exists.
func TestAdminService_Initialize_NotFirstRunAfterAdmin(t *testing.T) {
	svc := newAdminSvc(t)
	mustCreateAdmin(t, svc, "admin", "Password1!ab")

	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize after admin exists: %v", err)
	}
	if svc.IsFirstRun() {
		t.Error("IsFirstRun() = true when admins exist, want false")
	}
}

// ── ValidateSetupToken ────────────────────────────────────────────────────────

// TestAdminService_ValidateSetupToken_Valid verifies a freshly generated token
// is accepted.
func TestAdminService_ValidateSetupToken_Valid(t *testing.T) {
	svc := newAdminSvc(t)
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	token := svc.GetSetupToken()
	if !svc.ValidateSetupToken(token) {
		t.Error("ValidateSetupToken(valid token) = false, want true")
	}
}

// TestAdminService_ValidateSetupToken_Invalid verifies a wrong token is rejected.
func TestAdminService_ValidateSetupToken_Invalid(t *testing.T) {
	svc := newAdminSvc(t)
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if svc.ValidateSetupToken("wrongtoken") {
		t.Error("ValidateSetupToken(wrong) = true, want false")
	}
}

// ── CreateAdmin ───────────────────────────────────────────────────────────────

// TestAdminService_CreateAdmin_Success verifies basic admin creation.
func TestAdminService_CreateAdmin_Success(t *testing.T) {
	svc := newAdminSvc(t)
	admin, err := svc.CreateAdmin("alice", "Password1!ab", true)
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if admin.ID == 0 {
		t.Error("CreateAdmin returned ID=0")
	}
	if admin.Username != "alice" {
		t.Errorf("Username = %q, want %q", admin.Username, "alice")
	}
	if !admin.IsPrimary {
		t.Error("IsPrimary = false, want true")
	}
}

// TestAdminService_CreateAdmin_DuplicateUsernameReturnsError verifies that a
// duplicate username is rejected.
func TestAdminService_CreateAdmin_DuplicateUsernameReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	mustCreateAdmin(t, svc, "alice", "Password1!ab")
	_, err := svc.CreateAdmin("alice", "AnotherPass1!", false)
	if err == nil {
		t.Error("CreateAdmin duplicate: expected error, got nil")
	}
}

// TestAdminService_CreateAdmin_InvalidPasswordReturnsError verifies that a weak
// password triggers a validation error before any DB write.
func TestAdminService_CreateAdmin_InvalidPasswordReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	_, err := svc.CreateAdmin("bob", "weak", false)
	if err == nil {
		t.Error("CreateAdmin weak password: expected error, got nil")
	}
}

// ── CreateAdminWithSetupToken ─────────────────────────────────────────────────

// TestAdminService_CreateAdminWithSetupToken_Success verifies that a valid
// setup token allows creating the first admin.
func TestAdminService_CreateAdminWithSetupToken_Success(t *testing.T) {
	svc := newAdminSvc(t)
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	token := svc.GetSetupToken()

	admin, err := svc.CreateAdminWithSetupToken(token, "firstadmin", "Password1!ab")
	if err != nil {
		t.Fatalf("CreateAdminWithSetupToken: %v", err)
	}
	if admin.Username != "firstadmin" {
		t.Errorf("Username = %q, want %q", admin.Username, "firstadmin")
	}
}

// TestAdminService_CreateAdminWithSetupToken_InvalidToken rejects a wrong token.
func TestAdminService_CreateAdminWithSetupToken_InvalidToken(t *testing.T) {
	svc := newAdminSvc(t)
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	_, err := svc.CreateAdminWithSetupToken("badtoken", "admin", "Password1!ab")
	if err == nil {
		t.Error("CreateAdminWithSetupToken bad token: expected error, got nil")
	}
}

// ── Authenticate ──────────────────────────────────────────────────────────────

// TestAdminService_Authenticate_Success verifies correct credentials are accepted.
func TestAdminService_Authenticate_Success(t *testing.T) {
	svc := newAdminSvc(t)
	mustCreateAdmin(t, svc, "alice", "Password1!ab")

	admin, err := svc.Authenticate("alice", "Password1!ab")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if admin.Username != "alice" {
		t.Errorf("Authenticate returned Username = %q, want %q", admin.Username, "alice")
	}
}

// TestAdminService_Authenticate_WrongPassword rejects wrong password.
func TestAdminService_Authenticate_WrongPassword(t *testing.T) {
	svc := newAdminSvc(t)
	mustCreateAdmin(t, svc, "alice", "Password1!ab")

	_, err := svc.Authenticate("alice", "WrongPass1!a")
	if err == nil {
		t.Error("Authenticate wrong password: expected error, got nil")
	}
}

// TestAdminService_Authenticate_UnknownUser rejects non-existent username.
func TestAdminService_Authenticate_UnknownUser(t *testing.T) {
	svc := newAdminSvc(t)
	_, err := svc.Authenticate("nobody", "Password1!ab")
	if err == nil {
		t.Error("Authenticate unknown user: expected error, got nil")
	}
}

// ── ChangePassword ────────────────────────────────────────────────────────────

// TestAdminService_ChangePassword_Success verifies a valid current password allows
// a password change, and the new password then authenticates.
func TestAdminService_ChangePassword_Success(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	if err := svc.ChangePassword(admin.ID, "Password1!ab", "NewPass1!abc"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Authenticate with new password
	if _, err := svc.Authenticate("alice", "NewPass1!abc"); err != nil {
		t.Errorf("Authenticate after password change: %v", err)
	}
}

// TestAdminService_ChangePassword_WrongCurrentReturnsError verifies that a wrong
// current password blocks the change.
func TestAdminService_ChangePassword_WrongCurrentReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	err := svc.ChangePassword(admin.ID, "WrongCurrent1!", "NewPass1!abc")
	if err == nil {
		t.Error("ChangePassword wrong current: expected error, got nil")
	}
}

// ── GetAdmin ──────────────────────────────────────────────────────────────────

// TestAdminService_GetAdmin_Existing verifies GetAdmin returns the correct admin.
func TestAdminService_GetAdmin_Existing(t *testing.T) {
	svc := newAdminSvc(t)
	created := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	fetched, err := svc.GetAdmin(created.ID)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if fetched.Username != "alice" {
		t.Errorf("GetAdmin Username = %q, want %q", fetched.Username, "alice")
	}
}

// TestAdminService_GetAdmin_MissingReturnsError verifies GetAdmin fails for an
// unknown ID.
func TestAdminService_GetAdmin_MissingReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	_, err := svc.GetAdmin(9999)
	if err == nil {
		t.Error("GetAdmin missing: expected error, got nil")
	}
}

// ── GetAdminCount ──────────────────────────────────────────────────────────────

// TestAdminService_GetAdminCount_Empty verifies count is 0 on empty DB.
func TestAdminService_GetAdminCount_Empty(t *testing.T) {
	svc := newAdminSvc(t)
	count, err := svc.GetAdminCount()
	if err != nil {
		t.Fatalf("GetAdminCount: %v", err)
	}
	if count != 0 {
		t.Errorf("GetAdminCount empty = %d, want 0", count)
	}
}

// TestAdminService_GetAdminCount_AfterCreate verifies count increases.
func TestAdminService_GetAdminCount_AfterCreate(t *testing.T) {
	svc := newAdminSvc(t)
	mustCreateAdmin(t, svc, "alice", "Password1!ab")
	mustCreateAdmin(t, svc, "bob", "Password1!ab")

	count, err := svc.GetAdminCount()
	if err != nil {
		t.Fatalf("GetAdminCount: %v", err)
	}
	if count != 2 {
		t.Errorf("GetAdminCount = %d, want 2", count)
	}
}

// ── GenerateInviteToken ───────────────────────────────────────────────────────

// TestAdminService_GenerateInviteToken_ReturnsToken verifies token is non-empty.
func TestAdminService_GenerateInviteToken_ReturnsToken(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	token, err := svc.GenerateInviteToken(admin.ID)
	if err != nil {
		t.Fatalf("GenerateInviteToken: %v", err)
	}
	if token == "" {
		t.Error("GenerateInviteToken: returned empty token")
	}
}

// ── CreateAPIToken / ValidateAPIToken / GetAPITokenInfo / RegenerateAPIToken ──

// TestAdminService_APIToken_LifeCycle covers create, validate, info, regenerate.
func TestAdminService_APIToken_LifeCycle(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	// Create
	token, err := svc.CreateAPIToken(admin.ID, "default", "*")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	if token == "" {
		t.Fatal("CreateAPIToken: empty token")
	}

	// Validate
	adminID, err := svc.ValidateAPIToken(token)
	if err != nil {
		t.Fatalf("ValidateAPIToken: %v", err)
	}
	if adminID != admin.ID {
		t.Errorf("ValidateAPIToken adminID = %d, want %d", adminID, admin.ID)
	}

	// GetAPITokenInfo
	prefix, _, useCount, err := svc.GetAPITokenInfo(admin.ID)
	if err != nil {
		t.Fatalf("GetAPITokenInfo: %v", err)
	}
	if prefix == "" {
		t.Error("GetAPITokenInfo: empty prefix")
	}
	if useCount < 1 {
		t.Errorf("GetAPITokenInfo useCount = %d, want ≥1", useCount)
	}

	// Regenerate
	newToken, err := svc.RegenerateAPIToken(admin.ID)
	if err != nil {
		t.Fatalf("RegenerateAPIToken: %v", err)
	}
	if newToken == "" || newToken == token {
		t.Errorf("RegenerateAPIToken returned same or empty token")
	}
}

// TestAdminService_ValidateAPIToken_InvalidReturnsError verifies bad token fails.
func TestAdminService_ValidateAPIToken_InvalidReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	_, err := svc.ValidateAPIToken("badtoken")
	if err == nil {
		t.Error("ValidateAPIToken bad token: expected error, got nil")
	}
}

// TestAdminService_GetAPITokenInfo_NoToken verifies empty result when no token.
func TestAdminService_GetAPITokenInfo_NoToken(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	prefix, _, count, err := svc.GetAPITokenInfo(admin.ID)
	if err != nil {
		t.Fatalf("GetAPITokenInfo no token: %v", err)
	}
	if prefix != "" || count != 0 {
		t.Errorf("GetAPITokenInfo no token: prefix=%q count=%d, want empty/0", prefix, count)
	}
}

// ── CreateAdminInvite / ValidateInviteToken / CreateAdminWithInvite ───────────

// TestAdminService_InviteFlow_FullCycle covers create invite → validate → use.
func TestAdminService_InviteFlow_FullCycle(t *testing.T) {
	svc := newAdminSvc(t)
	primary := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	// Create invite for "bob"
	token, err := svc.CreateAdminInvite(primary.ID, "bob", time.Hour)
	if err != nil {
		t.Fatalf("CreateAdminInvite: %v", err)
	}
	if token == "" {
		t.Fatal("CreateAdminInvite: empty token")
	}

	// Validate
	invite, err := svc.ValidateInviteToken(token)
	if err != nil {
		t.Fatalf("ValidateInviteToken: %v", err)
	}
	if invite.Username != "bob" {
		t.Errorf("ValidateInviteToken Username = %q, want %q", invite.Username, "bob")
	}

	// Use to create admin
	bob, err := svc.CreateAdminWithInvite(token, "bob", "Password1!ab")
	if err != nil {
		t.Fatalf("CreateAdminWithInvite: %v", err)
	}
	if bob.Username != "bob" {
		t.Errorf("CreateAdminWithInvite Username = %q, want %q", bob.Username, "bob")
	}
}

// TestAdminService_ValidateInviteToken_InvalidReturnsError checks bad token.
func TestAdminService_ValidateInviteToken_InvalidReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	_, err := svc.ValidateInviteToken("badtoken")
	if err == nil {
		t.Error("ValidateInviteToken bad token: expected error, got nil")
	}
}

// TestAdminService_ValidateInviteToken_UsedReturnsError verifies a used token
// is rejected.
func TestAdminService_ValidateInviteToken_UsedReturnsError(t *testing.T) {
	svc := newAdminSvc(t)
	primary := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	token, err := svc.CreateAdminInvite(primary.ID, "carol", time.Hour)
	if err != nil {
		t.Fatalf("CreateAdminInvite: %v", err)
	}

	// Use the token once
	if _, err := svc.CreateAdminWithInvite(token, "carol", "Password1!ab"); err != nil {
		t.Fatalf("CreateAdminWithInvite: %v", err)
	}

	// Second use must fail
	_, err = svc.ValidateInviteToken(token)
	if err == nil {
		t.Error("ValidateInviteToken after use: expected error, got nil")
	}
}

// TestAdminService_CreateAdminWithInvite_UsernameMismatch verifies mismatched
// username is rejected.
func TestAdminService_CreateAdminWithInvite_UsernameMismatch(t *testing.T) {
	svc := newAdminSvc(t)
	primary := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	token, err := svc.CreateAdminInvite(primary.ID, "dave", time.Hour)
	if err != nil {
		t.Fatalf("CreateAdminInvite: %v", err)
	}

	_, err = svc.CreateAdminWithInvite(token, "wrongname", "Password1!ab")
	if err == nil {
		t.Error("CreateAdminWithInvite username mismatch: expected error, got nil")
	}
}

// ── ListPendingInvites / RevokeInvite / CleanupExpiredInvites ─────────────────

// TestAdminService_ListPendingInvites verifies pending invites are listed.
func TestAdminService_ListPendingInvites(t *testing.T) {
	svc := newAdminSvc(t)
	primary := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	if _, err := svc.CreateAdminInvite(primary.ID, "eve", time.Hour); err != nil {
		t.Fatalf("CreateAdminInvite: %v", err)
	}

	invites, err := svc.ListPendingInvites()
	if err != nil {
		t.Fatalf("ListPendingInvites: %v", err)
	}
	if len(invites) == 0 {
		t.Error("ListPendingInvites: expected ≥1, got 0")
	}
}

// TestAdminService_RevokeInvite verifies revoking an invite removes it from
// the pending list.
func TestAdminService_RevokeInvite(t *testing.T) {
	svc := newAdminSvc(t)
	primary := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	if _, err := svc.CreateAdminInvite(primary.ID, "frank", time.Hour); err != nil {
		t.Fatalf("CreateAdminInvite: %v", err)
	}

	invites, err := svc.ListPendingInvites()
	if err != nil || len(invites) == 0 {
		t.Fatalf("ListPendingInvites: err=%v count=%d", err, len(invites))
	}

	if err := svc.RevokeInvite(invites[0].ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}

	invites2, err := svc.ListPendingInvites()
	if err != nil {
		t.Fatalf("ListPendingInvites after revoke: %v", err)
	}
	if len(invites2) != 0 {
		t.Errorf("ListPendingInvites after revoke: count = %d, want 0", len(invites2))
	}
}

// TestAdminService_CleanupExpiredInvites verifies expired invites are removed.
func TestAdminService_CleanupExpiredInvites(t *testing.T) {
	svc := newAdminSvc(t)
	primary := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	// Create an already-expired invite (negative duration)
	if _, err := svc.CreateAdminInvite(primary.ID, "grace", -time.Minute); err != nil {
		t.Fatalf("CreateAdminInvite expired: %v", err)
	}

	deleted, err := svc.CleanupExpiredInvites()
	if err != nil {
		t.Fatalf("CleanupExpiredInvites: %v", err)
	}
	if deleted == 0 {
		t.Error("CleanupExpiredInvites: expected ≥1 deleted, got 0")
	}
}

// ── GenerateRecoveryKeys / ValidateRecoveryKey / GetRecoveryKeysStatus ────────

// TestAdminService_RecoveryKeys_LifeCycle covers generate → validate → status.
func TestAdminService_RecoveryKeys_LifeCycle(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	keys, err := svc.GenerateRecoveryKeys(admin.ID)
	if err != nil {
		t.Fatalf("GenerateRecoveryKeys: %v", err)
	}
	if len(keys) != 10 {
		t.Errorf("GenerateRecoveryKeys: count = %d, want 10", len(keys))
	}

	// Validate the first key
	ok, err := svc.ValidateRecoveryKey(admin.ID, keys[0])
	if err != nil {
		t.Fatalf("ValidateRecoveryKey: %v", err)
	}
	if !ok {
		t.Error("ValidateRecoveryKey valid key: expected true, got false")
	}

	// Re-using the same key should fail (it's consumed)
	ok2, err := svc.ValidateRecoveryKey(admin.ID, keys[0])
	if err != nil {
		t.Fatalf("ValidateRecoveryKey reuse: %v", err)
	}
	if ok2 {
		t.Error("ValidateRecoveryKey reused key: expected false, got true")
	}

	// Status
	status, err := svc.GetRecoveryKeysStatus(admin.ID)
	if err != nil {
		t.Fatalf("GetRecoveryKeysStatus: %v", err)
	}
	if status.Total != 10 {
		t.Errorf("GetRecoveryKeysStatus Total = %d, want 10", status.Total)
	}
	if status.Used != 1 {
		t.Errorf("GetRecoveryKeysStatus Used = %d, want 1", status.Used)
	}
	if status.Remaining != 9 {
		t.Errorf("GetRecoveryKeysStatus Remaining = %d, want 9", status.Remaining)
	}
}

// TestAdminService_ValidateRecoveryKey_Invalid verifies bad key returns false.
func TestAdminService_ValidateRecoveryKey_Invalid(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	ok, err := svc.ValidateRecoveryKey(admin.ID, "0000-0000-0000-0000")
	if err != nil {
		t.Fatalf("ValidateRecoveryKey bad key: %v", err)
	}
	if ok {
		t.Error("ValidateRecoveryKey bad key: expected false, got true")
	}
}

// ── CleanupExpiredSessions / CleanupExpiredTokens ─────────────────────────────

// TestAdminService_CleanupExpiredSessions_Empty verifies no error on empty table.
func TestAdminService_CleanupExpiredSessions_Empty(t *testing.T) {
	svc := newAdminSvc(t)
	if err := svc.CleanupExpiredSessions(); err != nil {
		t.Errorf("CleanupExpiredSessions empty: %v", err)
	}
}

// TestAdminService_CleanupExpiredSessions_RemovesExpired inserts an expired
// session and verifies cleanup removes it.
func TestAdminService_CleanupExpiredSessions_RemovesExpired(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	// Insert an already-expired session
	past := time.Now().Add(-time.Hour)
	_, err := svc.db.Exec(`INSERT INTO admin_sessions (id, admin_id, expires_at) VALUES (?, ?, ?)`,
		"sess-expired", admin.ID, past)
	if err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	if err := svc.CleanupExpiredSessions(); err != nil {
		t.Errorf("CleanupExpiredSessions: %v", err)
	}

	var count int
	svc.db.QueryRow("SELECT COUNT(*) FROM admin_sessions").Scan(&count)
	if count != 0 {
		t.Errorf("sessions after cleanup = %d, want 0", count)
	}
}

// TestAdminService_CleanupExpiredTokens verifies expired setup and API tokens
// are removed.
func TestAdminService_CleanupExpiredTokens(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	// Insert an expired setup token
	past := time.Now().Add(-time.Hour)
	_, err := svc.db.Exec(`INSERT INTO setup_tokens (token, purpose, expires_at) VALUES (?, ?, ?)`,
		"expired-setup-tok", "reset", past)
	if err != nil {
		t.Fatalf("insert expired setup token: %v", err)
	}

	// Insert an expired API token
	_, err = svc.db.Exec(`INSERT INTO api_tokens (admin_id, name, token_hash, token_prefix, expires_at) VALUES (?, ?, ?, ?, ?)`,
		admin.ID, "expired", "exphash", "expp", past)
	if err != nil {
		t.Fatalf("insert expired api token: %v", err)
	}

	if err := svc.CleanupExpiredTokens(); err != nil {
		t.Errorf("CleanupExpiredTokens: %v", err)
	}
}

// ── TOTP: EnableTOTP / DisableTOTP / GetTOTPSecret / GetTOTPBackupCodes / UseBackupCode

// TestAdminService_TOTP_LifeCycle covers enable → get-secret → get-codes → use-code → disable.
func TestAdminService_TOTP_LifeCycle(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	codes := []string{"AAAA-BBBB-CCCC-DDDD", "1111-2222-3333-4444"}
	if err := svc.EnableTOTP(admin.ID, "JBSWY3DPEHPK3PXP", codes); err != nil {
		t.Fatalf("EnableTOTP: %v", err)
	}

	// GetTOTPSecret
	secret, err := svc.GetTOTPSecret(admin.ID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("GetTOTPSecret = %q, want %q", secret, "JBSWY3DPEHPK3PXP")
	}

	// GetTOTPBackupCodes
	fetched, err := svc.GetTOTPBackupCodes(admin.ID)
	if err != nil {
		t.Fatalf("GetTOTPBackupCodes: %v", err)
	}
	if len(fetched) != 2 {
		t.Errorf("GetTOTPBackupCodes count = %d, want 2", len(fetched))
	}

	// UseBackupCode — valid
	used, err := svc.UseBackupCode(admin.ID, "AAAA-BBBB-CCCC-DDDD")
	if err != nil {
		t.Fatalf("UseBackupCode: %v", err)
	}
	if !used {
		t.Error("UseBackupCode valid: expected true, got false")
	}

	// UseBackupCode — already consumed
	used2, err := svc.UseBackupCode(admin.ID, "AAAA-BBBB-CCCC-DDDD")
	if err != nil {
		t.Fatalf("UseBackupCode reuse: %v", err)
	}
	if used2 {
		t.Error("UseBackupCode reuse: expected false, got true")
	}

	// DisableTOTP
	if err := svc.DisableTOTP(admin.ID); err != nil {
		t.Fatalf("DisableTOTP: %v", err)
	}

	// GetTOTPSecret after disable
	_, err = svc.GetTOTPSecret(admin.ID)
	if err == nil {
		t.Error("GetTOTPSecret after disable: expected error, got nil")
	}
}

// TestAdminService_GetTOTPSecret_NotConfigured verifies the error when no TOTP is set.
func TestAdminService_GetTOTPSecret_NotConfigured(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	_, err := svc.GetTOTPSecret(admin.ID)
	if err == nil {
		t.Error("GetTOTPSecret not configured: expected error, got nil")
	}
}

// TestAdminService_GetTOTPBackupCodes_NoCodes verifies empty slice when no codes set.
func TestAdminService_GetTOTPBackupCodes_NoCodes(t *testing.T) {
	svc := newAdminSvc(t)
	admin := mustCreateAdmin(t, svc, "alice", "Password1!ab")

	codes, err := svc.GetTOTPBackupCodes(admin.ID)
	if err != nil {
		t.Fatalf("GetTOTPBackupCodes no codes: %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("GetTOTPBackupCodes no codes: count = %d, want 0", len(codes))
	}
}

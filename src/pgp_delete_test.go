// SPDX-License-Identifier: MIT
// Coverage tests for the pgp keypair delete flow (AI.md PART 12).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/pgp"
)

// TestPgpDeleteCore_Success generates a keypair, deletes it, and verifies both
// key files are gone, publishing is disabled, and the PGP key URL is cleared.
func TestPgpDeleteCore_Success(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	if _, _, _, _, err := pgpGenerateCore(cfgDir, dataDir); err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}

	paths := config.GetAppPaths(cfgDir, dataDir)
	secDir := pgp.SecurityDir(paths.Config)
	if _, err := os.Stat(filepath.Join(secDir, pgp.PrivateKeyFile)); err != nil {
		t.Fatalf("private key not present before delete: %v", err)
	}

	done, err := pgpDeleteCore(cfgDir, dataDir, func() bool { return true })
	if err != nil {
		t.Fatalf("pgpDeleteCore: %v", err)
	}
	if !done {
		t.Fatal("expected done=true on confirmed delete")
	}

	if _, err := os.Stat(filepath.Join(secDir, pgp.PrivateKeyFile)); !os.IsNotExist(err) {
		t.Fatalf("private key still present after delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(secDir, pgp.PublicKeyFile)); !os.IsNotExist(err) {
		t.Fatalf("public key still present after delete: %v", err)
	}

	cfg, _, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg.Web.Security.PublishPGPKey {
		t.Fatal("expected PublishPGPKey disabled after delete")
	}
	if cfg.Web.Security.PGPKeyURL != "" {
		t.Fatalf("expected PGPKeyURL cleared, got %q", cfg.Web.Security.PGPKeyURL)
	}
}

// TestPgpDeleteCore_MarksRevoked verifies the DB row is marked revoked while the
// fingerprint stays for audit history (AI.md PART 12 "revoked").
func TestPgpDeleteCore_MarksRevoked(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	kp, _, _, _, err := pgpGenerateCore(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}

	if _, err := pgpDeleteCore(cfgDir, dataDir, func() bool { return true }); err != nil {
		t.Fatalf("pgpDeleteCore: %v", err)
	}

	paths := config.GetAppPaths(cfgDir, dataDir)
	dbMgr, err := database.NewMigrationManager(filepath.Join(paths.Data, "db", "server.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer dbMgr.Close()
	meta, err := pgp.GetKeypairMeta(dbMgr.GetDB())
	if err != nil {
		t.Fatalf("GetKeypairMeta: %v", err)
	}
	if meta == nil {
		t.Fatal("expected keypair meta row to survive delete for audit history")
	}
	if !meta.Revoked {
		t.Fatal("expected keypair marked revoked after delete")
	}
	if meta.Fingerprint != kp.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", meta.Fingerprint, kp.Fingerprint)
	}
}

// TestPgpDeleteCore_EmitsAudit configures a real audit log, deletes the keypair,
// and verifies a security.private_key_deleted event carrying the fingerprint is
// written (AI.md PART 12 — the fingerprint stays in audit history).
func TestPgpDeleteCore_EmitsAudit(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	kp, _, _, _, err := pgpGenerateCore(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}

	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	auditFile := filepath.Join(t.TempDir(), "audit.log")
	cfg.Server.Logs.Audit.Enabled = true
	cfg.Server.Logs.Audit.Filename = auditFile
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	if _, err := pgpDeleteCore(cfgDir, dataDir, func() bool { return true }); err != nil {
		t.Fatalf("pgpDeleteCore: %v", err)
	}

	data, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(data), "security.private_key_deleted") {
		t.Fatalf("audit log missing delete event: %s", data)
	}
	if !strings.Contains(string(data), kp.Fingerprint) {
		t.Fatalf("audit log missing fingerprint %q: %s", kp.Fingerprint, data)
	}
}

// TestPgpDelete_TypedConfirmation drives the full pgpDelete wrapper through the
// real promptDeleteConfirm/promptLine path by feeding "DELETE" on stdin, covering
// the interactive typed-confirmation gate (AI.md PART 12) without a terminal. The
// success branch returns normally (no os.Exit), so it is safe to call in-process.
func TestPgpDelete_TypedConfirmation(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	if _, _, _, _, err := pgpGenerateCore(cfgDir, dataDir); err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString("DELETE\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	pgpDelete(cfgDir, dataDir)

	paths := config.GetAppPaths(cfgDir, dataDir)
	secDir := pgp.SecurityDir(paths.Config)
	if _, err := os.Stat(filepath.Join(secDir, pgp.PrivateKeyFile)); !os.IsNotExist(err) {
		t.Fatalf("private key still present after confirmed delete: %v", err)
	}
}

// TestPgpDeleteCore_Declined verifies a declined confirmation leaves the keypair
// intact and reports done=false.
func TestPgpDeleteCore_Declined(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	if _, _, _, _, err := pgpGenerateCore(cfgDir, dataDir); err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}

	done, err := pgpDeleteCore(cfgDir, dataDir, func() bool { return false })
	if err != nil {
		t.Fatalf("pgpDeleteCore: %v", err)
	}
	if done {
		t.Fatal("expected done=false when confirmation is declined")
	}

	paths := config.GetAppPaths(cfgDir, dataDir)
	if _, err := os.Stat(filepath.Join(pgp.SecurityDir(paths.Config), pgp.PrivateKeyFile)); err != nil {
		t.Fatalf("private key should remain after declined delete: %v", err)
	}
	cfg, _, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !cfg.Web.Security.PublishPGPKey {
		t.Fatal("expected PublishPGPKey untouched after declined delete")
	}
}

// TestPgpDeleteCore_DBOpenError verifies a database-open failure is surfaced and
// nothing is deleted. A directory occupying the server.db path makes the SQLite
// open fail.
func TestPgpDeleteCore_DBOpenError(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	paths := config.GetAppPaths(cfgDir, dataDir)
	dbPath := filepath.Join(paths.Data, "db", "server.db")
	if err := os.RemoveAll(dbPath); err != nil {
		t.Fatalf("remove existing db: %v", err)
	}
	if err := os.MkdirAll(dbPath, 0o700); err != nil {
		t.Fatalf("occupy db path with a directory: %v", err)
	}

	done, err := pgpDeleteCore(cfgDir, dataDir, func() bool { return true })
	if err == nil {
		t.Fatal("expected error when the database cannot be opened")
	}
	if done {
		t.Fatal("expected done=false on database error")
	}
}

// TestPgpDeleteCore_NoKeypair verifies delete is idempotent when no keypair
// exists: it confirms, removes nothing, and does not error on the missing DB row.
func TestPgpDeleteCore_NoKeypair(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")

	done, err := pgpDeleteCore(cfgDir, dataDir, func() bool { return true })
	if err != nil {
		t.Fatalf("pgpDeleteCore with no keypair: %v", err)
	}
	if !done {
		t.Fatal("expected done=true even with no keypair present")
	}
}

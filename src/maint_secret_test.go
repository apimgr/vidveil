package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/google/uuid"
)

func TestHandleMaintenanceSecretArgumentValidation(t *testing.T) {
	for _, arg := range []string{"", "rotate", "generate installation_secret", "rotate installation_secret extra"} {
		err := handleMaintenanceSecret(arg, t.TempDir(), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "Usage:") {
			t.Errorf("arg %q: got %v, want usage error", arg, err)
		}
	}
}

func TestRotateEncryptionKeyReencryptsAllAtRestData(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	cfg, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	oldKey := cfg.Server.Security.EncryptionKey
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	db := mgr.GetDB()
	ownerID := "test-owner"
	if _, err := db.Exec(`INSERT INTO owners (id) VALUES (?)`, ownerID); err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	rawToken := "own_" + strings.Repeat("a", 32)
	hash := apitoken.HashToken(rawToken)
	storedHash, err := apitoken.EncryptHash(oldKey, hash)
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	storedToken, err := apitoken.EncryptToken(oldKey, rawToken)
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	tokenID, err := uuid.NewV7()
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO api_tokens (id, owner_id, kind, token_hash, token_prefix, token_last4, token_enc) VALUES (?, ?, 'primary', ?, ?, ?, ?)`, tokenID.String(), ownerID, storedHash, rawToken[:12], rawToken[len(rawToken)-4:], storedToken); err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	reportBody, err := secreport.EncryptAESGCM(oldKey, []byte("sensitive report"))
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	contact, err := secreport.EncryptAESGCM(oldKey, []byte(`{"email":"researcher@example.com"}`))
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO security_reports (tracking_id, id_window, severity, component, body_enc, body_alg, contact_enc, token_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "test-report", 1, "low", "test", reportBody, string(secreport.EncryptionMethodAES), contact, "test-hash"); err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatal(err)
	}

	if err := rotateEncryptionKey(configDir, dataDir); err != nil {
		t.Fatal(err)
	}
	updated, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	newKey := updated.Server.Security.EncryptionKey
	if newKey == oldKey {
		t.Fatal("encryption-key rotation kept the previous key")
	}
	mgr, err = openSecretDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mgr.Close() }()
	var rotatedHash string
	if err := mgr.GetDB().QueryRow(`SELECT token_hash FROM api_tokens WHERE id = ?`, tokenID.String()).Scan(&rotatedHash); err != nil {
		t.Fatal(err)
	}
	if got, err := apitoken.DecryptHash(newKey, rotatedHash); err != nil || got != hash {
		t.Fatalf("rotated API-token digest = (%q, %v), want original digest", got, err)
	}
	var rotatedToken string
	if err := mgr.GetDB().QueryRow(`SELECT token_enc FROM api_tokens WHERE id = ?`, tokenID.String()).Scan(&rotatedToken); err != nil {
		t.Fatal(err)
	}
	if got, err := apitoken.DecryptToken(newKey, rotatedToken); err != nil || got != rawToken {
		t.Fatalf("rotated API-token copy = (%q, %v), want original token", got, err)
	}
	var rotatedBody []byte
	if err := mgr.GetDB().QueryRow(`SELECT body_enc FROM security_reports WHERE tracking_id = ?`, "test-report").Scan(&rotatedBody); err != nil {
		t.Fatal(err)
	}
	plain, err := secreport.DecryptAESGCM(newKey, rotatedBody)
	if err != nil || string(plain) != "sensitive report" {
		t.Fatalf("rotated report body = (%q, %v), want original body", plain, err)
	}
	var rotatedContact []byte
	if err := mgr.GetDB().QueryRow(`SELECT contact_enc FROM security_reports WHERE tracking_id = ?`, "test-report").Scan(&rotatedContact); err != nil {
		t.Fatal(err)
	}
	contactPlain, err := secreport.DecryptAESGCM(newKey, rotatedContact)
	if err != nil || string(contactPlain) != `{"email":"researcher@example.com"}` {
		t.Fatalf("rotated report contact = (%q, %v), want original contact", contactPlain, err)
	}
	previousKey, valid, err := secret.LoadPreviousEncryptionKey(context.Background(), mgr.GetDB(), newKey, time.Now().UTC())
	if err != nil || !valid || previousKey != oldKey {
		t.Fatal("rotation did not preserve previous encryption key for the grace period")
	}
}

func TestHandleMaintenanceSecretRejectsAutomaticSecrets(t *testing.T) {
	for _, name := range []string{"cookie_signing_key", "csrf_token_secret", "unknown"} {
		err := handleMaintenanceSecret("rotate "+name, t.TempDir(), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "unsupported secret name") {
			t.Errorf("name %q: got %v, want rejection", name, err)
		}
	}
}

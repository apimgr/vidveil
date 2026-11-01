package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/secreport"
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
	hash := apitoken.HashToken("tok_rotate_test")
	storedHash, err := apitoken.EncryptHash(oldKey, hash)
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	storedToken, err := apitoken.EncryptToken(oldKey, "tok_0123456789abcdefghijkmnpqrstuv")
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO api_tokens (token_hash, token_prefix, resource_type, resource_id, token_enc) VALUES (?, ?, ?, ?, ?)`, storedHash, "tok_rotate", "test", "resource", storedToken); err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	reportBody, err := secreport.EncryptAESGCM(oldKey, []byte("sensitive report"))
	if err != nil {
		_ = mgr.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO security_reports (tracking_id, severity, component, summary, encrypted_body, encryption_method, credit_preference) VALUES (?, ?, ?, ?, ?, ?, ?)`, "test-report", "low", "test", "summary", reportBody, string(secreport.EncryptionMethodAES), "anonymous"); err != nil {
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
	if err := mgr.GetDB().QueryRow(`SELECT token_hash FROM api_tokens WHERE resource_id = ?`, "resource").Scan(&rotatedHash); err != nil {
		t.Fatal(err)
	}
	if got, err := apitoken.DecryptHash(newKey, rotatedHash); err != nil || got != hash {
		t.Fatalf("rotated API-token digest = (%q, %v), want original digest", got, err)
	}
	var rotatedToken string
	if err := mgr.GetDB().QueryRow(`SELECT token_enc FROM api_tokens WHERE resource_id = ?`, "resource").Scan(&rotatedToken); err != nil {
		t.Fatal(err)
	}
	if got, err := apitoken.DecryptToken(newKey, rotatedToken); err != nil || got != "tok_0123456789abcdefghijkmnpqrstuv" {
		t.Fatalf("rotated API-token copy = (%q, %v), want original token", got, err)
	}
	var rotatedBody []byte
	if err := mgr.GetDB().QueryRow(`SELECT encrypted_body FROM security_reports WHERE tracking_id = ?`, "test-report").Scan(&rotatedBody); err != nil {
		t.Fatal(err)
	}
	plain, err := secreport.DecryptAESGCM(newKey, rotatedBody)
	if err != nil || string(plain) != "sensitive report" {
		t.Fatalf("rotated report body = (%q, %v), want original body", plain, err)
	}
	var previousKey string
	if err := mgr.GetDB().QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key = ?`, "security.previous_encryption_key").Scan(&previousKey); err != nil {
		t.Fatal(err)
	}
	if previousKey != oldKey {
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

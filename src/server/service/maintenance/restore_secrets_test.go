// SPDX-License-Identifier: MIT
package maintenance

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/common/dbstore"
	"github.com/apimgr/vidveil/src/server/service/database"
)

func TestRestoreWritesAppSecretsIntoRestoredSQLiteDatabase(t *testing.T) {
	manager, tempDir := newMaintMgrWithTempDirs(t)
	backupPath := filepath.Join(tempDir, "app-secrets-restore.tar.gz")
	if err := manager.BackupWithOptions(BackupOptions{Filename: backupPath}); err != nil {
		t.Fatalf("create backup: %v", err)
	}

	restoreRoot := filepath.Join(tempDir, "restore")
	restored := NewMaintenanceManager(filepath.Join(restoreRoot, "config"), filepath.Join(restoreRoot, "data"), "1.0.0")
	restored.paths.Backup = manager.paths.Backup
	if err := restored.Restore(backupPath); err != nil {
		t.Fatalf("restore backup: %v", err)
	}

	db, err := database.NewAppDatabase(database.DatabaseConfig{
		Driver: database.DriverSQLite,
		Path:   filepath.Join(restored.paths.Data, "db", "server.db"),
	})
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer db.Close()
	var value string
	if err := db.Read(context.Background(), func(ctx context.Context, reader dbstore.Reader) error {
		return reader.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = 'installation_secret'`).Scan(&value)
	}); err != nil {
		t.Fatalf("read restored installation secret: %v", err)
	}
	if value == "" {
		t.Fatal("restored installation_secret is empty")
	}
}

func TestWriteApplicationSecretsPreservesExistingRowsWhenOnlyIfEmpty(t *testing.T) {
	tempRoot := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		t.Fatal(err)
	}
	dbDir, err := os.MkdirTemp(tempRoot, "vidveil-restore-secrets-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dbDir); err != nil {
			t.Errorf("remove temporary database directory: %v", err)
		}
	})
	db, err := database.NewAppDatabase(database.DatabaseConfig{
		Driver: database.DriverSQLite,
		Path:   filepath.Join(dbDir, "server.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.Write(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE app_secrets (key TEXT PRIMARY KEY, value TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO app_secrets (key, value) VALUES ('installation_secret', 'live-secret')`)
		return err
	}); err != nil {
		t.Fatalf("initialize app_secrets: %v", err)
	}

	inserted, err := writeApplicationSecrets(db, map[string]string{
		"installation_secret": "archive-secret",
		"cookie_signing_key":  "archive-cookie",
	}, true)
	if err != nil {
		t.Fatalf("write only-if-empty secrets: %v", err)
	}
	if inserted {
		t.Fatal("writeApplicationSecrets reported replacing populated rows")
	}
	var liveSecret string
	if err := db.Read(context.Background(), func(ctx context.Context, reader dbstore.Reader) error {
		return reader.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = 'installation_secret'`).Scan(&liveSecret)
	}); err != nil {
		t.Fatalf("read live installation secret: %v", err)
	}
	if liveSecret != "live-secret" {
		t.Fatalf("installation_secret = %q, want live-secret", liveSecret)
	}
}

func TestPreserveLiveRestoreKeyMaterialKeepsFilesOutsideArchiveSnapshot(t *testing.T) {
	tempRoot := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		t.Fatal(err)
	}
	configDir, err := os.MkdirTemp(tempRoot, "vidveil-restore-key-material-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(configDir); err != nil {
			t.Errorf("remove temporary key directory: %v", err)
		}
	})
	securityDir := filepath.Join(configDir, "security")
	if err := os.MkdirAll(securityDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(securityDir, "pgp.pub.asc"), []byte("live-public-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(securityDir, "keyservers.state"), []byte("live-state"), 0600); err != nil {
		t.Fatal(err)
	}

	files, err := preserveLiveRestoreKeyMaterial(filterRestoreKeyMaterial([]restoreFileEntry{
		{name: "config/security/pgp.pub.asc", content: []byte("archive-public-key")},
		{name: "config/security/server.yml", content: []byte("archive-security-config")},
	}), configDir)
	if err != nil {
		t.Fatalf("preserve live key material: %v", err)
	}
	got := make(map[string]string, len(files))
	for _, file := range files {
		got[file.name] = string(file.content)
	}
	if got["config/security/pgp.pub.asc"] != "live-public-key" {
		t.Fatalf("restored public key = %q, want live key", got["config/security/pgp.pub.asc"])
	}
	if got["config/security/keyservers.state"] != "live-state" {
		t.Fatalf("restored keyserver state = %q, want live state", got["config/security/keyservers.state"])
	}
	if got["config/security/server.yml"] != "archive-security-config" {
		t.Fatal("unrelated archived security configuration was not kept")
	}
}

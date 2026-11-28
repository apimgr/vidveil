// SPDX-License-Identifier: MIT
package maintenance

import (
	"context"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/database"
)

func attachTestApplicationSecrets(t *testing.T, manager *MaintenanceManager) {
	t.Helper()
	databaseDir := filepath.Join(manager.paths.Data, "db")
	if err := os.MkdirAll(databaseDir, 0700); err != nil {
		t.Fatalf("create backup test database directory: %v", err)
	}
	db, err := database.NewAppDatabase(database.DatabaseConfig{Driver: database.DriverSQLite, Path: filepath.Join(databaseDir, "server.db")})
	if err != nil {
		t.Fatalf("create backup test database: %v", err)
	}
	if err := db.Write(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE app_secrets (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			rotated_at DATETIME,
			expires_at DATETIME,
			previous_value TEXT
		)`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO app_secrets (key, value) VALUES ('installation_secret', ?)`, base64.StdEncoding.EncodeToString([]byte("test installation secret with enough entropy")))
		return err
	}); err != nil {
		_ = db.Close()
		t.Fatalf("initialize backup test database: %v", err)
	}
	manager.SetDatabase(db)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close backup test database: %v", err)
		}
	})
}

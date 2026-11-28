// SPDX-License-Identifier: MIT
package maintenance

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/apimgr/vidveil/src/common/dbstore"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/pgp"
)

func (m *MaintenanceManager) prepareRestoredSecrets(files []restoreFileEntry, tempDir string) ([]restoreFileEntry, func() error, error) {
	var secretFile *restoreFileEntry
	for i := range files {
		if files[i].name == "app_secrets.json" {
			secretFile = &files[i]
			break
		}
	}
	if secretFile == nil {
		return nil, nil, fmt.Errorf("backup is missing app_secrets.json")
	}
	secretBytes, err := restoreEntryBytes(*secretFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read backed up app_secrets.json: %w", err)
	}
	var secrets map[string]string
	if err := json.Unmarshal(secretBytes, &secrets); err != nil {
		return nil, nil, fmt.Errorf("decode backed up app_secrets.json: %w", err)
	}
	encodedInstallationSecret := strings.TrimSpace(secrets["installation_secret"])
	if encodedInstallationSecret == "" {
		return nil, nil, fmt.Errorf("backed up app_secrets.json is missing installation_secret")
	}
	if _, err := base64.StdEncoding.DecodeString(encodedInstallationSecret); err != nil {
		return nil, nil, fmt.Errorf("backed up installation_secret is invalid")
	}

	archiveFingerprint, err := restoreArchivePGPFingerprint(files, tempDir)
	if err != nil {
		return nil, nil, err
	}
	filtered := make([]restoreFileEntry, 0, len(files))
	for _, file := range files {
		if file.name == "app_secrets.json" {
			continue
		}
		filtered = append(filtered, file)
	}

	if m.appDB != nil && m.appDB.Driver() != database.DriverSQLite {
		currentSecrets, err := readApplicationSecrets(m.appDB)
		if err != nil {
			return nil, nil, fmt.Errorf("read live remote application secrets: %w", err)
		}
		if len(currentSecrets) == 0 {
			return filtered, func() error {
				_, err := writeApplicationSecrets(m.appDB, secrets, true)
				return err
			}, nil
		}
		currentMeta, err := pgp.GetKeypairMeta(m.appDB)
		if err != nil {
			return nil, nil, fmt.Errorf("read live remote PGP fingerprint: %w", err)
		}
		currentFingerprint := ""
		if currentMeta != nil {
			currentFingerprint = currentMeta.Fingerprint
		}
		if currentSecrets["installation_secret"] != encodedInstallationSecret || currentFingerprint != archiveFingerprint {
			filtered, err = preserveLiveRestoreKeyMaterial(filterRestoreKeyMaterial(filtered), m.paths.Config)
			if err != nil {
				return nil, nil, err
			}
			m.audit("restore.key_material_kept", "failure", map[string]interface{}{
				"reason": "installation_secret_or_pgp_fingerprint_mismatch",
			})
			fmt.Println("WARN restore.key_material_kept: remote application secrets or PGP fingerprint differ; archived key material was not installed")
		}
		return filtered, func() error { return nil }, nil
	}

	for _, file := range filtered {
		if file.name != "data/db/server.db" && file.name != "config/server.db" {
			continue
		}
		db, err := database.NewAppDatabase(database.DatabaseConfig{
			Driver: database.DriverSQLite, Path: file.contentPath, JournalMode: "WAL",
		})
		if err != nil {
			return nil, nil, fmt.Errorf("open restored SQLite snapshot for application secrets: %w", err)
		}
		_, writeErr := writeApplicationSecrets(db, secrets, false)
		closeErr := db.Close()
		if writeErr != nil || closeErr != nil {
			return nil, nil, fmt.Errorf("restore application secrets into SQLite snapshot: %w", errors.Join(writeErr, closeErr))
		}
		return filtered, func() error { return nil }, nil
	}
	dataRootReplaced := false
	for _, file := range filtered {
		if strings.HasPrefix(file.name, "data/") {
			dataRootReplaced = true
			break
		}
	}
	if !dataRootReplaced && m.appDB != nil && m.appDB.Driver() == database.DriverSQLite {
		return filtered, func() error {
			_, err := writeApplicationSecrets(m.appDB, secrets, false)
			return err
		}, nil
	}

	return filtered, func() error {
		manager, err := openRestoredDatabase(m.paths.Config, m.paths.Data)
		if err != nil {
			return err
		}
		defer manager.Close()
		targetDB := manager.GetAppDatabase()
		_, err = writeApplicationSecrets(targetDB, secrets, targetDB.Driver() != database.DriverSQLite)
		return err
	}, nil
}

func restoreEntryBytes(file restoreFileEntry) ([]byte, error) {
	if file.contentPath != "" {
		return os.ReadFile(file.contentPath)
	}
	return file.content, nil
}

func readApplicationSecrets(db *database.AppDatabase) (map[string]string, error) {
	secrets := make(map[string]string)
	if err := db.Read(context.Background(), func(ctx context.Context, reader dbstore.Reader) error {
		rows, err := reader.QueryContext(ctx, "SELECT key, value FROM app_secrets ORDER BY key")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				return err
			}
			secrets[key] = value
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}
	return secrets, nil
}

func writeApplicationSecrets(db *database.AppDatabase, secrets map[string]string, onlyIfEmpty bool) (bool, error) {
	keys := make([]string, 0, len(secrets))
	for key, value := range secrets {
		if strings.TrimSpace(key) == "" || value == "" {
			return false, fmt.Errorf("backup contains an empty application secret")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	inserted := false
	err := db.Write(context.Background(), func(tx *sql.Tx) error {
		if onlyIfEmpty {
			var count int
			if err := tx.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM app_secrets").Scan(&count); err != nil {
				return err
			}
			if count != 0 {
				return nil
			}
		}
		for _, key := range keys {
			if _, err := tx.ExecContext(context.Background(), `INSERT INTO app_secrets (key, value, created_at)
				VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, secrets[key]); err != nil {
				return err
			}
		}
		inserted = true
		return nil
	})
	return inserted, err
}

func restoreArchivePGPFingerprint(files []restoreFileEntry, tempDir string) (string, error) {
	for _, file := range files {
		if file.name != "config/security/pgp.pub.asc" {
			continue
		}
		data, err := restoreEntryBytes(file)
		if err != nil {
			return "", fmt.Errorf("read backed up PGP public key: %w", err)
		}
		stagedConfig := filepath.Join(tempDir, "restore-key-fingerprint")
		securityDir := pgp.SecurityDir(stagedConfig)
		if err := os.MkdirAll(securityDir, 0700); err != nil {
			return "", fmt.Errorf("prepare backed up PGP fingerprint: %w", err)
		}
		if err := os.WriteFile(filepath.Join(securityDir, pgp.StagedPublicKeyFile), data, 0600); err != nil {
			return "", fmt.Errorf("stage backed up PGP public key: %w", err)
		}
		fingerprint, err := pgp.StagedFingerprint(stagedConfig)
		if err != nil {
			return "", fmt.Errorf("read backed up PGP fingerprint: %w", err)
		}
		return fingerprint, nil
	}
	return "", nil
}

func filterRestoreKeyMaterial(files []restoreFileEntry) []restoreFileEntry {
	filtered := make([]restoreFileEntry, 0, len(files))
	for _, file := range files {
		if strings.HasPrefix(file.name, "config/security/pgp.") || file.name == "config/security/keyservers.state" {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func preserveLiveRestoreKeyMaterial(files []restoreFileEntry, configDir string) ([]restoreFileEntry, error) {
	securityDir := pgp.SecurityDir(configDir)
	entries, err := os.ReadDir(securityDir)
	if os.IsNotExist(err) {
		return files, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect live PGP key material: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "pgp.") && name != "keyservers.state" {
			continue
		}
		path := filepath.Join(securityDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect live PGP key material %q: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("live PGP key material %q is not a regular file", name)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read live PGP key material %q: %w", name, err)
		}
		files = append(files, restoreFileEntry{
			name:    filepath.ToSlash(filepath.Join("config", "security", name)),
			mode:    int64(info.Mode().Perm()),
			content: content,
		})
	}
	return files, nil
}

func openRestoredDatabase(configDir, dataDir string) (*database.SchemaManager, error) {
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return nil, fmt.Errorf("load restored database configuration: %w", err)
	}
	paths := config.GetAppPaths(configDir, dataDir)
	sqliteDir := appConfig.Server.Database.SQLite.Dir
	if sqliteDir == "" {
		sqliteDir = config.GetDatabaseDir(paths.Data)
	}
	serverDB := appConfig.Server.Database.SQLite.ServerDB
	if serverDB == "" {
		serverDB = "server.db"
	}
	manager, err := database.NewSchemaManagerWithConfig(database.DatabaseConfig{
		Driver:      database.Driver(appConfig.Server.Database.Driver),
		URL:         appConfig.Server.Database.URL,
		Token:       appConfig.Server.Database.Token,
		Path:        filepath.Join(sqliteDir, serverDB),
		JournalMode: appConfig.Server.Database.SQLite.JournalMode,
		BusyTimeout: appConfig.Server.Database.SQLite.BusyTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("open restored database: %w", err)
	}
	return manager, nil
}

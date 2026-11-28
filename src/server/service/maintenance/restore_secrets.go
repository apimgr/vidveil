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
	"time"

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
		var snapshot *restoreFileEntry
		remoteFiles := make([]restoreFileEntry, 0, len(filtered))
		for i := range filtered {
			if filtered[i].name == "data/db/server.db" || filtered[i].name == "config/server.db" {
				snapshot = &filtered[i]
				continue
			}
			remoteFiles = append(remoteFiles, filtered[i])
		}
		filtered = remoteFiles
		currentSecrets, err := readApplicationSecrets(m.appDB)
		if err != nil {
			return nil, nil, fmt.Errorf("read live remote application secrets: %w", err)
		}
		empty, err := remoteDatabaseIsEmpty(m.appDB)
		if err != nil {
			return nil, nil, fmt.Errorf("check live remote database before restore: %w", err)
		}
		if empty {
			return filtered, func() error {
				if snapshot != nil {
					if err := seedRemoteDatabaseFromSQLite(m.appDB, snapshot.contentPath); err != nil {
						return err
					}
				}
				_, err := writeApplicationSecrets(m.appDB, secrets, true)
				return err
			}, nil
		}
		if len(currentSecrets) == 0 {
			// The remote database has rows but no app_secrets. Keep it authoritative
			// rather than importing an archive into an already-used database.
			filtered = filterRestoreKeyMaterial(filtered)
			return filtered, func() error { return nil }, nil
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

const restoreDatabaseBatchSize = 500

var restoreDatabaseTables = []string{
	"owners", "settings", "scheduler_tasks", "scheduler_history", "captcha_challenges",
	"notifications", "pgp_keypair", "security_reports", "security_thanks", "favorites",
	"api_tokens", "health_stats", "health_request_buckets", "audit_log",
}

func remoteDatabaseIsEmpty(db *database.AppDatabase) (bool, error) {
	for _, table := range restoreDatabaseTables {
		remoteTable := restoreRemoteTableName(db.Driver(), table)
		var count int
		err := db.Read(context.Background(), func(ctx context.Context, reader dbstore.Reader) error {
			query := "SELECT COUNT(*) FROM " + remoteTable
			if table == "owners" {
				query += " WHERE id <> 'system'"
			}
			return reader.QueryRowContext(ctx, query).Scan(&count)
		})
		if err != nil {
			return false, err
		}
		if count != 0 {
			return false, nil
		}
	}
	return true, nil
}

func restoreRemoteTableName(driver database.Driver, table string) string {
	if driver != database.DriverSQLite && table == "audit_log" {
		return "srv_audit_log"
	}
	return table
}

func seedRemoteDatabaseFromSQLite(target *database.AppDatabase, snapshotPath string) (retErr error) {
	if snapshotPath == "" {
		return fmt.Errorf("remote database seed snapshot has no content path")
	}
	if err := claimRemoteDatabaseForRestoreSeed(target); err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, target.WriteRestoreSeedBatch(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec(`DELETE FROM restore_seed_locks WHERE lock_id = 'remote_seed'`)
			return err
		}))
	}()
	source, err := database.NewAppDatabase(database.DatabaseConfig{
		Driver: database.DriverSQLite, Path: snapshotPath, JournalMode: "WAL",
	})
	if err != nil {
		return fmt.Errorf("open archived SQLite database for remote seed: %w", err)
	}
	defer func() {
		retErr = errors.Join(retErr, source.Close())
	}()

	for _, table := range restoreDatabaseTables {
		columns, err := restoreTableColumns(source, table)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return fmt.Errorf("inspect archived %s table: %w", table, err)
		}
		targetTable := restoreRemoteTableName(target.Driver(), table)
		targetColumns, err := restoreTableColumns(target, targetTable)
		if err != nil {
			return fmt.Errorf("inspect remote %s table: %w", targetTable, err)
		}
		columns = intersectRestoreColumns(columns, targetColumns)
		if len(columns) == 0 {
			continue
		}
		if err := copyRestoreTable(source, target, table, targetTable, columns); err != nil {
			return fmt.Errorf("seed remote %s table: %w", table, err)
		}
	}
	return nil
}

func claimRemoteDatabaseForRestoreSeed(target *database.AppDatabase) error {
	return target.Write(context.Background(), func(tx *sql.Tx) error {
		var active int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM restore_seed_locks WHERE lock_id = 'remote_seed' AND lease_until > CURRENT_TIMESTAMP`).Scan(&active); err != nil {
			return err
		}
		if active != 0 {
			return database.ErrRestoreSeedInProgress
		}
		for _, table := range restoreDatabaseTables {
			remoteTable := restoreRemoteTableName(target.Driver(), table)
			query := "SELECT COUNT(*) FROM " + remoteTable
			if table == "owners" {
				query += " WHERE id <> 'system'"
			}
			var count int
			if err := tx.QueryRow(query).Scan(&count); err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf("remote database became populated before restore seeding")
			}
		}
		_, err := tx.Exec(`INSERT INTO restore_seed_locks (lock_id, lease_until) VALUES ('remote_seed', datetime('now', '+30 minutes')) ON CONFLICT(lock_id) DO UPDATE SET lease_until = excluded.lease_until`)
		return err
	})
}

func restoreTableColumns(db *database.AppDatabase, table string) ([]string, error) {
	columns := make([]string, 0)
	err := db.Read(context.Background(), func(ctx context.Context, reader dbstore.Reader) error {
		rows, err := reader.QueryContext(ctx, "SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			columns = append(columns, name)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, sql.ErrNoRows
	}
	return columns, nil
}

func intersectRestoreColumns(source, target []string) []string {
	targetSet := make(map[string]struct{}, len(target))
	for _, column := range target {
		targetSet[column] = struct{}{}
	}
	columns := make([]string, 0, len(source))
	for _, column := range source {
		if _, ok := targetSet[column]; ok {
			columns = append(columns, column)
		}
	}
	return columns
}

func copyRestoreTable(source, target *database.AppDatabase, sourceTable, targetTable string, columns []string) error {
	quotedColumns := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, column := range columns {
		quotedColumns[i] = `"` + strings.ReplaceAll(column, `"`, `""`) + `"`
		placeholders[i] = "?"
	}
	insert := "INSERT OR IGNORE INTO " + targetTable + " (" + strings.Join(quotedColumns, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ")"
	for offset := 0; ; offset += restoreDatabaseBatchSize {
		batch := make([][]interface{}, 0, restoreDatabaseBatchSize)
		err := source.Read(context.Background(), func(ctx context.Context, reader dbstore.Reader) error {
			rows, err := reader.QueryContext(ctx, "SELECT "+strings.Join(quotedColumns, ",")+" FROM "+sourceTable+" ORDER BY rowid LIMIT ? OFFSET ?", restoreDatabaseBatchSize, offset)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				values := make([]interface{}, len(columns))
				pointers := make([]interface{}, len(columns))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err := rows.Scan(pointers...); err != nil {
					return err
				}
				batch = append(batch, values)
			}
			return rows.Err()
		})
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		if err := target.WriteRestoreSeedBatch(context.Background(), func(tx *sql.Tx) error {
			for _, values := range batch {
				if _, err := tx.Exec(insert, values...); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if len(batch) < restoreDatabaseBatchSize {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
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

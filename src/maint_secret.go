// SPDX-License-Identifier: MIT
// AI.md PART 11: sensitive project-secret rotation.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/pgp"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
)

// handleMaintenanceSecret implements --maintenance secret rotate <name>.
func handleMaintenanceSecret(arg, configDir, dataDir string) error {
	fields := strings.Fields(arg)
	binaryName := filepath.Base(os.Args[0])
	usage := fmt.Sprintf("Usage: %s --maintenance secret rotate <installation_secret|encryption_key>", binaryName)
	if len(fields) != 2 || fields[0] != "rotate" {
		return fmt.Errorf("invalid secret action or arguments\n   %s", usage)
	}
	name := fields[1]
	if name != "installation_secret" && name != "encryption_key" {
		return fmt.Errorf("unsupported secret name %q (cookie_signing_key and csrf_token_secret are auto-rotated only)\n   %s", name, usage)
	}
	if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
		return err
	}
	if promptLine("Type ROTATE to confirm: ") != "ROTATE" {
		return fmt.Errorf("rotation cancelled: typed confirmation did not match ROTATE")
	}

	var err error
	switch name {
	case "installation_secret":
		err = rotateInstallationSecret(configDir, dataDir)
	case "encryption_key":
		err = rotateEncryptionKey(configDir, dataDir)
	}
	if err != nil {
		return fmt.Errorf("secret rotation failed: %w", err)
	}
	fmt.Println(terminal.StatusIcon(true) + " Secret rotated: " + name)
	return nil
}

func openSecretDB(dataDir string) (*database.MigrationManager, error) {
	mgr, err := database.NewMigrationManager(filepath.Join(config.GetAppPaths("", dataDir).Data, "db", "server.db"))
	if err != nil {
		return nil, err
	}
	if err := mgr.RunMigrations(); err != nil {
		mgr.Close()
		return nil, err
	}
	return mgr, nil
}

func rotateInstallationSecret(configDir, dataDir string) error {
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer mgr.Close()
	ctx := context.Background()
	secrets := secret.NewManager(mgr.GetDB())
	if err := secrets.EnsureSecrets(ctx); err != nil {
		return err
	}
	old, err := secrets.GetInstallationSecret(ctx)
	if err != nil {
		return err
	}
	priv, privErr := pgp.LoadPrivateKey(configDir, old)
	if privErr != nil && !os.IsNotExist(privErr) {
		return fmt.Errorf("decrypt PGP private key: %w", privErr)
	}
	if err := secrets.Rotate(ctx, secret.InstallationSecret); err != nil {
		return err
	}
	newSecret, err := secrets.GetInstallationSecret(ctx)
	if err != nil {
		return err
	}
	if privErr == nil {
		kp, _, _, err := pgp.ParsePrivateKey(priv)
		if err != nil {
			return fmt.Errorf("parse PGP private key: %w", err)
		}
		if err := pgp.WriteKeypair(configDir, kp, newSecret); err != nil {
			return fmt.Errorf("re-encrypt PGP private key: %w", err)
		}
	}
	return emitSecretRotationAudit(configDir, dataDir, "security.installation_secret_rotated")
}

func rotateEncryptionKey(configDir, dataDir string) error {
	cfg, path, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	old := cfg.Server.Security.EncryptionKey
	if len(old) == 0 {
		return fmt.Errorf("server.security.encryption_key is not configured")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	newKey := hex.EncodeToString(key)
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer mgr.Close()
	db := mgr.GetDB()
	// Re-encrypt AES-backed report bodies in one transaction. PGP rows are
	// intentionally untouched: their key material is independent of this key.
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	rows, err := tx.Query("SELECT tracking_id, encrypted_body FROM security_reports WHERE encryption_method = ?", string(secreport.EncryptionMethodAES))
	if err != nil {
		tx.Rollback()
		return err
	}
	type reportBlob struct {
		id   string
		body []byte
	}
	var reports []reportBlob
	for rows.Next() {
		var r reportBlob
		if err := rows.Scan(&r.id, &r.body); err != nil {
			rows.Close()
			tx.Rollback()
			return err
		}
		reports = append(reports, r)
	}
	rows.Close()
	for _, r := range reports {
		plain, err := secreport.DecryptAESGCM(old, r.body)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("decrypt report %s: %w", r.id, err)
		}
		enc, err := secreport.EncryptAESGCM(newKey, plain)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec("UPDATE security_reports SET encrypted_body = ?, updated_at = CURRENT_TIMESTAMP WHERE tracking_id = ?", enc, r.id); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO settings(key,value,type,updated_at) VALUES(?,?,?,CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=CURRENT_TIMESTAMP", "security.previous_encryption_key", old, "secret"); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	cfg.Server.Security.EncryptionKey = newKey
	if err := config.SaveAppConfig(cfg, path); err != nil {
		return err
	}
	return emitSecretRotationAudit(configDir, dataDir, "security.encryption_key_rotated")
}

func emitSecretRotationAudit(configDir, dataDir, event string) error {
	cfg, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return fmt.Errorf("audit log unavailable: %w", err)
	}
	logger, err := logging.NewAppLogger(cfg)
	if err != nil {
		return fmt.Errorf("audit log unavailable: %w", err)
	}
	defer logger.Close()
	operator := "root"
	if u, e := user.Current(); e == nil {
		operator = u.Username
	}
	logger.Audit(event, operator, "operator", localOperatorIP(), "success", map[string]interface{}{"grace_period": map[string]string{"security.installation_secret_rotated": "7d", "security.encryption_key_rotated": "30d"}[event]})
	return nil
}

var _ *sql.DB

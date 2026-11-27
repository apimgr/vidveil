// SPDX-License-Identifier: MIT
// AI.md PART 11: sensitive project-secret rotation.
package app

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
	"github.com/apimgr/vidveil/src/server/service/apitoken"
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
		_ = mgr.Close()
		return nil, err
	}
	return mgr, nil
}

func rotateInstallationSecret(configDir, dataDir string) error {
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	secrets := secret.NewManager(mgr.GetAppDatabase())
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
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()
	db := mgr.GetAppDatabase()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if unlock := secreport.LockKeyRotation(); unlock != nil {
		defer unlock()
	}
	if err := secret.ImportLegacyPreviousEncryptionKey(ctx, db, old); err != nil {
		return err
	}
	if exists, err := secret.PreviousEncryptionKeyExists(ctx, db); err != nil {
		return err
	} else if exists {
		previous, valid, err := secret.LoadPreviousEncryptionKey(ctx, db, old, time.Now().UTC())
		if err != nil {
			return err
		}
		if !valid {
			previous = ""
		}
		if err := completeEncryptionKeyRotation(ctx, db, old, previous); err != nil {
			return fmt.Errorf("cannot rotate again until previous encryption-key data is complete: %w", err)
		}
		if err := db.Write(ctx, func(tx *sql.Tx) error {
			return secret.DeletePreviousEncryptionKey(ctx, tx)
		}); err != nil {
			return err
		}
	}
	newBytes := make([]byte, 32)
	if _, err := rand.Read(newBytes); err != nil {
		return err
	}
	newKey := hex.EncodeToString(newBytes)
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_prefix = token_prefix WHERE id = ''`); err != nil {
			return err
		}
		return secret.StorePreviousEncryptionKey(ctx, tx, old, newKey, time.Now().UTC().Add(30*24*time.Hour))
	}); err != nil {
		return fmt.Errorf("persist wrapped previous encryption key: %w", err)
	}
	cfg.Server.Security.EncryptionKey = newKey
	if cfg.Server.Security.EncryptionKeyVersion < 1 {
		cfg.Server.Security.EncryptionKeyVersion = 1
	} else {
		cfg.Server.Security.EncryptionKeyVersion++
	}
	if err := config.SaveAppConfig(cfg, path); err != nil {
		return fmt.Errorf("save rotated encryption key; staged recovery state retained: %w", err)
	}
	if err := completeEncryptionKeyRotation(ctx, db, newKey, old); err != nil {
		return fmt.Errorf("rotation key saved; eager re-encryption will resume at startup: %w", err)
	}
	if err := emitSecretRotationAudit(configDir, dataDir, "security.encryption_key_rotated"); err != nil {
		return err
	}
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		return secret.DeletePendingEncryptionKey(ctx, tx)
	}); err != nil {
		return err
	}
	return nil
}

// completeEncryptionKeyRotation eagerly rewrites any AES data still readable
// only with the wrapped grace-period key. It is safe to rerun after a crash.
func completeEncryptionKeyRotation(ctx context.Context, db *database.AppDatabase, currentKey, previousKey string) error {
	const batch = 100
	for offset := 0; ; offset += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		count := 0
		err := db.Write(ctx, func(tx *sql.Tx) error {
			rows, err := tx.QueryContext(ctx, `SELECT id, token_hash, token_enc FROM api_tokens ORDER BY id LIMIT ? OFFSET ?`, batch, offset)
			if err != nil {
				return fmt.Errorf("read owner-token encryption state: %w", err)
			}
			type item struct{ id, hash, token string }
			items := make([]item, 0, batch)
			for rows.Next() {
				var value item
				if err := rows.Scan(&value.id, &value.hash, &value.token); err != nil {
					_ = rows.Close()
					return err
				}
				items = append(items, value)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			count = len(items)
			for _, value := range items {
				if hash, usedPrevious, err := apitoken.DecryptHashWithPrevious(currentKey, previousKey, value.hash); err != nil {
					return fmt.Errorf("decrypt API token digest %s during recovery: %w", value.id, err)
				} else if usedPrevious {
					if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ? WHERE id = ?`, hash, value.id); err != nil {
						return err
					}
				}
				if value.token != "" {
					token, usedPrevious, err := apitoken.DecryptTokenWithPrevious(currentKey, previousKey, value.token)
					if err != nil {
						return fmt.Errorf("decrypt API token %s during recovery: %w", value.id, err)
					}
					if usedPrevious {
						encrypted, err := apitoken.EncryptToken(currentKey, token)
						if err != nil {
							return err
						}
						if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_enc = ? WHERE id = ?`, encrypted, value.id); err != nil {
							return err
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if count < batch {
			break
		}
		if err := yieldRotationBatch(ctx); err != nil {
			return err
		}
	}

	for _, field := range []struct{ column, label string }{{"body_enc", "security report"}, {"contact_enc", "security contact"}} {
		for offset := 0; ; offset += batch {
			if err := ctx.Err(); err != nil {
				return err
			}
			count := 0
			err := db.Write(ctx, func(tx *sql.Tx) error {
				query := `SELECT tracking_id, ` + field.column + ` FROM security_reports WHERE ` + field.column + ` IS NOT NULL`
				if field.column == "body_enc" {
					query += ` AND body_alg = ? ORDER BY tracking_id LIMIT ? OFFSET ?`
				} else {
					query += ` ORDER BY tracking_id LIMIT ? OFFSET ?`
				}
				var rows *sql.Rows
				var err error
				if field.column == "body_enc" {
					rows, err = tx.QueryContext(ctx, query, string(secreport.EncryptionMethodAES), batch, offset)
				} else {
					rows, err = tx.QueryContext(ctx, query, batch, offset)
				}
				if err != nil {
					return fmt.Errorf("read %s during recovery: %w", field.label, err)
				}
				type blob struct {
					id   string
					data []byte
				}
				items := make([]blob, 0, batch)
				for rows.Next() {
					var value blob
					if err := rows.Scan(&value.id, &value.data); err != nil {
						_ = rows.Close()
						return err
					}
					items = append(items, value)
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return err
				}
				if err := rows.Close(); err != nil {
					return err
				}
				count = len(items)
				for _, value := range items {
					_, err := secreport.DecryptAESGCM(currentKey, value.data)
					if err == nil {
						continue
					}
					if previousKey == "" {
						return fmt.Errorf("decrypt %s %s with current encryption key: %w", field.label, value.id, err)
					}
					plain, previousErr := secreport.DecryptAESGCM(previousKey, value.data)
					if previousErr != nil {
						return fmt.Errorf("decrypt %s %s with current or previous encryption key: %w", field.label, value.id, previousErr)
					}
					encrypted, err := secreport.EncryptAESGCM(currentKey, plain)
					if err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, `UPDATE security_reports SET `+field.column+` = ?, updated_at = CURRENT_TIMESTAMP WHERE tracking_id = ?`, encrypted, value.id); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			if count < batch {
				break
			}
			if err := yieldRotationBatch(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func yieldRotationBatch(ctx context.Context) error {
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

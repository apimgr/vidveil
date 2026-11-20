// SPDX-License-Identifier: MIT
package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const bearerAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func ensureGeneratedConfigSecrets(configDir string, cfg *AppConfig) error {
	if cfg == nil {
		return nil
	}
	generated := make(map[string]string)
	if cfg.Server.Token == "" {
		token, err := newBearerToken()
		if err != nil {
			return err
		}
		cfg.Server.Token = token
		cfg.GeneratedServerToken = true
		generated["server.token"] = token
	}
	if cfg.Server.Security.EncryptionKey == "" {
		cfg.Server.Security.EncryptionKey = generateToken(32)
		cfg.Server.Security.EncryptionKeyVersion = 1
		generated["server.security.encryption_key"] = cfg.Server.Security.EncryptionKey
	}
	for _, role := range []struct {
		name     string
		webhooks *map[string]string
	}{
		{"admin", &cfg.Server.Contact.Admin.Webhooks},
		{"security", &cfg.Server.Contact.Security.Webhooks},
		{"abuse", &cfg.Server.Contact.Abuse.Webhooks},
		{"general", &cfg.Server.Contact.General.Webhooks},
	} {
		if *role.webhooks == nil {
			continue
		}
		for name, endpoint := range *role.webhooks {
			secretName := name + "_secret"
			if strings.HasSuffix(name, "_secret") || strings.TrimSpace(endpoint) == "" || strings.TrimSpace((*role.webhooks)[secretName]) != "" {
				continue
			}
			secret := generateToken(32)
			(*role.webhooks)[secretName] = secret
			generated["server.contact."+role.name+".webhooks."+secretName] = secret
		}
	}
	for _, token := range []struct {
		name  string
		value *string
	}{
		{"server.metrics.auth.tokens.prometheus", &cfg.Server.Metrics.Auth.Tokens.Prometheus},
		{"server.metrics.auth.tokens.grafana", &cfg.Server.Metrics.Auth.Tokens.Grafana},
		{"server.metrics.auth.tokens.loki", &cfg.Server.Metrics.Auth.Tokens.Loki},
	} {
		if *token.value != "" {
			continue
		}
		value, err := newBearerToken()
		if err != nil {
			return err
		}
		*token.value = value
		generated[token.name] = value
	}
	if len(generated) == 0 && (cfg.Server.Security.EncryptionKey == "" || cfg.Server.Security.EncryptionKeyVersion != 0) {
		return nil
	}
	keys := make([]string, 0, len(generated))
	for key := range generated {
		if key != "server.security.encryption_key_version" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	cfg.GeneratedSecretKeys = keys
	secretsPath := filepath.Join(configDir, "secrets.yml")
	secrets := make(map[string]interface{})
	if data, err := os.ReadFile(secretsPath); err == nil {
		secrets, err = decodeConfigMap(data)
		if err != nil {
			return fmt.Errorf("parse secrets.yml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read secrets.yml: %w", err)
	}
	for key, value := range generated {
		parts := strings.Split(key, ".")
		setPathValue(secrets, value, parts...)
	}
	if cfg.Server.Security.EncryptionKey != "" && cfg.Server.Security.EncryptionKeyVersion == 0 {
		cfg.Server.Security.EncryptionKeyVersion = 1
		setPathValue(secrets, 1, "server", "security", "encryption_key_version")
	}
	data, err := marshalYAML(secrets)
	if err != nil {
		return fmt.Errorf("encode secrets.yml: %w", err)
	}
	if err := atomicConfigWrite(secretsPath, data); err != nil {
		return fmt.Errorf("persist generated secrets: %w", err)
	}
	return nil
}
func newBearerToken() (string, error) {
	var token strings.Builder
	token.Grow(36)
	token.WriteString("tok_")
	limit := big.NewInt(int64(len(bearerAlphabet)))
	for range 32 {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate bearer token: %w", err)
		}
		token.WriteByte(bearerAlphabet[n.Int64()])
	}
	return token.String(), nil
}

func defaultBearerToken() string {
	token, err := newBearerToken()
	if err != nil {
		panic(fmt.Sprintf("config: failed to generate default operator token: %v", err))
	}
	return token
}

func readLegacySecretsFromBackup(configPath string) map[string]string {
	for _, path := range []string{configPath, configPath + ".bak"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		root, err := decodeConfigMap(data)
		if err != nil {
			continue
		}
		security, ok := pathValue(root, "server", "security")
		if !ok {
			continue
		}
		values, ok := security.(map[string]interface{})
		if !ok {
			continue
		}
		legacy := make(map[string]string)
		for _, key := range []string{"installation_secret", "installation_secret_previous", "installation_secret_previous_expires", "encryption_key_previous", "encryption_key_previous_expires"} {
			if value, exists := values[key]; exists && fmt.Sprint(value) != "" {
				legacy[key] = fmt.Sprint(value)
			}
		}
		if len(legacy) > 0 {
			return legacy
		}
	}
	return nil
}

func ImportLegacyAppSecrets(db *sql.DB, configPath string, cfg *AppConfig) error {
	if db == nil || cfg == nil || len(cfg.LegacyAppSecrets) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin legacy secret import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for key, value := range cfg.LegacyAppSecrets {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO app_secrets (key, value, created_at) VALUES (?, ?, ?)`, key, base64.StdEncoding.EncodeToString([]byte(value)), time.Now()); err != nil {
			return fmt.Errorf("import legacy app secret %s: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy secret import: %w", err)
	}
	if err := removeLegacySecretsFromFiles(configPath); err != nil {
		log.Printf("config: imported legacy app secrets but could not remove old config values: %v", err)
	}
	return nil
}

func removeLegacySecretsFromFiles(configPath string) error {
	paths := []string{configPath}
	dropDir := filepath.Join(filepath.Dir(configPath), "yaml.d")
	entries, err := os.ReadDir(dropDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}
		paths = append(paths, filepath.Join(dropDir, entry.Name()))
	}
	legacyNames := []string{"installation_secret", "installation_secret_previous", "installation_secret_previous_expires", "encryption_key_previous", "encryption_key_previous_expires"}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s after secret import: %w", path, readErr)
		}
		root, decodeErr := decodeConfigMap(data)
		if decodeErr != nil {
			return fmt.Errorf("parse %s after secret import: %w", path, decodeErr)
		}
		changed := false
		for _, key := range legacyNames {
			if deletePathValue(root, "server", "security", key) {
				changed = true
			}
		}
		if !changed {
			continue
		}
		encoded, encodeErr := marshalYAML(root)
		if encodeErr != nil {
			return encodeErr
		}
		if err := atomicConfigWrite(path, encoded); err != nil {
			return fmt.Errorf("remove imported legacy keys from %s: %w", path, err)
		}
	}
	return nil
}

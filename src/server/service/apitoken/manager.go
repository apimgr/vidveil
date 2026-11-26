// SPDX-License-Identifier: MIT
// Package apitoken implements anonymous owner-token lifecycle operations.
package apitoken

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/google/uuid"
)

const tokenAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// OwnerCookieName is the project-specific, stable browser credential name.
const OwnerCookieName = "vidveil_owner_token_ab5213f6"

// CookieMaxAge converts the configured sliding-cookie lifetime to seconds.
func CookieMaxAge(value string) int {
	const defaultAge = 365 * 24 * time.Hour
	if value == "" {
		return int(defaultAge.Seconds())
	}
	var duration time.Duration
	if strings.HasSuffix(value, "d") || strings.HasSuffix(value, "y") {
		unit := time.Hour * 24
		if strings.HasSuffix(value, "y") {
			unit *= 365
		}
		count, err := strconv.ParseInt(value[:len(value)-1], 10, 32)
		if err == nil && count > 0 && count <= int64((1<<63-1)/unit) {
			duration = unit * time.Duration(count)
		}
	} else if parsed, err := time.ParseDuration(value); err == nil {
		duration = parsed
	}
	if duration <= 0 {
		return int(defaultAge.Seconds())
	}
	return int(duration.Seconds())
}

// Manager owns API-token persistence and validation.
type Manager struct {
	db                      *sql.DB
	encryptionKey           string
	encryptionKeyMu         sync.RWMutex
	decryptionFailureLogger func(table, rowID string)
}

func (m *Manager) currentEncryptionKey() string {
	m.encryptionKeyMu.RLock()
	defer m.encryptionKeyMu.RUnlock()
	return m.encryptionKey
}

// SetEncryptionKey applies a live config reload after the DB key marker commits.
func (m *Manager) SetEncryptionKey(key string) {
	if m == nil {
		return
	}
	m.encryptionKeyMu.Lock()
	m.encryptionKey = key
	m.encryptionKeyMu.Unlock()
}

// SetDecryptionFailureLogger installs a sanitized security-event callback.
func (m *Manager) SetDecryptionFailureLogger(logger func(table, rowID string)) {
	if m != nil {
		m.decryptionFailureLogger = logger
	}
}

func (m *Manager) logDecryptionFailure(table, rowID string) {
	if m != nil && m.decryptionFailureLogger != nil {
		m.decryptionFailureLogger(table, rowID)
	}
}

// Principal is the owner resolved from a valid token.
type Principal struct {
	OwnerID string
	TokenID string
	Kind    string
	Prefix  string
}

// IssuedToken contains a raw token only at the issuance boundary.
type IssuedToken struct {
	OwnerID string
	TokenID string
	Token   string
	Prefix  string
}

// LinkedToken contains safe metadata; raw token values are never included.
type LinkedToken struct {
	ID        string
	Name      string
	Prefix    string
	Suffix    string
	CreatedAt int64
	LastUsed  sql.NullInt64
	ExpiresAt sql.NullInt64
	RevokedAt sql.NullInt64
}

// PrimaryTokenMetadata contains masked primary-token details for preferences views.
type PrimaryTokenMetadata struct {
	ID        string
	Prefix    string
	Suffix    string
	CreatedAt int64
	LastUsed  sql.NullInt64
}

// NewManager creates a token manager backed by the application database.
func NewManager(db *sql.DB, encryptionKey string) (*Manager, error) {
	if db == nil {
		return nil, fmt.Errorf("API-token database is required")
	}
	if encryptionKey == "" {
		return nil, fmt.Errorf("API-token encryption key is required")
	}
	return &Manager{db: db, encryptionKey: encryptionKey}, nil
}

// IssuePrimary creates an anonymous owner and its non-expiring primary token.
func (m *Manager) IssuePrimary(ctx context.Context) (IssuedToken, error) {
	ownerID, err := randomID()
	if err != nil {
		return IssuedToken{}, err
	}
	token, err := generateOwnerToken()
	if err != nil {
		return IssuedToken{}, err
	}
	prefix := token[:12]
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("begin owner token issuance: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_prefix = token_prefix WHERE id = ''`); err != nil {
		return IssuedToken{}, err
	}
	key := m.currentEncryptionKey()
	if err := secreport.ValidateCurrentEncryptionKey(ctx, tx, key); err != nil {
		return IssuedToken{}, err
	}
	storedHash, storedToken, err := m.encryptWithKey(token, key)
	if err != nil {
		return IssuedToken{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO owners(id) VALUES (?)`, ownerID); err != nil {
		return IssuedToken{}, fmt.Errorf("create token owner: %w", err)
	}
	tokenID, err := uuid.NewV7()
	if err != nil {
		return IssuedToken{}, fmt.Errorf("generate primary token id: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO api_tokens
		(id, owner_id, kind, name, token_hash, token_prefix, token_last4, token_enc)
		VALUES (?, ?, 'primary', NULL, ?, ?, ?, ?)`, tokenID.String(), ownerID, storedHash, prefix, token[len(token)-4:], storedToken)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("store primary token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IssuedToken{}, fmt.Errorf("commit owner token issuance: %w", err)
	}
	return IssuedToken{OwnerID: ownerID, TokenID: tokenID.String(), Token: token, Prefix: prefix}, nil
}

// Authenticate resolves an active owner token and updates bounded last-use data.
func (m *Manager) Authenticate(ctx context.Context, raw string) (Principal, error) {
	return m.authenticate(ctx, raw, true)
}

// ValidateOnly checks an owner token without changing last-use metadata.
func (m *Manager) ValidateOnly(ctx context.Context, raw string) (Principal, error) {
	return m.authenticate(ctx, raw, false)
}

func (m *Manager) authenticate(ctx context.Context, raw string, updateLastUsed bool) (Principal, error) {
	if !isOwnerToken(raw) {
		return Principal{}, ErrInvalidToken
	}
	want := HashToken(raw)
	rows, err := m.db.QueryContext(ctx, `SELECT id, owner_id, kind, token_hash, token_prefix, revoked_at, expires_at, last_used_at
		FROM api_tokens ORDER BY id`)
	if err != nil {
		return Principal{}, fmt.Errorf("read owner tokens: %w", err)
	}
	type tokenRow struct {
		id                        string
		owner, kind, hash, prefix string
		revoked, expires, used    sql.NullInt64
	}
	var found *tokenRow
	type hashRewrite struct{ id, value string }
	var rewrites []hashRewrite
	currentKey := m.currentEncryptionKey()
	for rows.Next() {
		var item tokenRow
		if err := rows.Scan(&item.id, &item.owner, &item.kind, &item.hash, &item.prefix, &item.revoked, &item.expires, &item.used); err != nil {
			_ = rows.Close()
			return Principal{}, fmt.Errorf("scan owner token: %w", err)
		}
		storedHash, usedPrevious, err := DecryptHashWithPrevious(currentKey, "", item.hash)
		if err != nil {
			previous, valid, previousErr := secret.LoadPreviousEncryptionKey(ctx, m.db, currentKey, time.Now().UTC())
			if previousErr != nil {
				_ = rows.Close()
				return Principal{}, previousErr
			}
			if valid {
				storedHash, usedPrevious, err = DecryptHashWithPrevious(currentKey, previous, item.hash)
			}
		}
		if err != nil {
			_ = rows.Close()
			m.logDecryptionFailure("api_tokens", item.id)
			return Principal{}, fmt.Errorf("validate stored owner token %s: %w", item.id, err)
		}
		if usedPrevious {
			rewrites = append(rewrites, hashRewrite{id: item.id, value: storedHash})
		}
		if subtle.ConstantTimeCompare([]byte(storedHash), []byte(want)) == 1 {
			copy := item
			found = &copy
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return Principal{}, fmt.Errorf("read owner tokens: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Principal{}, fmt.Errorf("close owner tokens: %w", err)
	}
	for _, rewrite := range rewrites {
		if _, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ? WHERE id = ?`, rewrite.value, rewrite.id); err != nil {
			return Principal{}, fmt.Errorf("rewrite owner-token digest %s with current key: %w", rewrite.id, err)
		}
	}
	now := time.Now().Unix()
	if found == nil || found.owner == "system" || found.revoked.Valid || (found.expires.Valid && found.expires.Int64 <= now) {
		return Principal{}, ErrInvalidToken
	}
	if updateLastUsed && (!found.used.Valid || found.used.Int64 <= now-60) {
		if _, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now, found.id); err != nil {
			return Principal{}, fmt.Errorf("update token use time: %w", err)
		}
		if _, err := m.db.ExecContext(ctx, `UPDATE owners SET last_seen_at = ? WHERE id = ?`, now, found.owner); err != nil {
			return Principal{}, fmt.Errorf("update owner use time: %w", err)
		}
	}
	return Principal{OwnerID: found.owner, TokenID: found.id, Kind: found.kind, Prefix: found.prefix}, nil
}

// CreateLinked issues a revocable owner-scoped token after validating primary authority.
func (m *Manager) CreateLinked(ctx context.Context, primary Principal, name string, expiresAt *int64, maxLinked int) (IssuedToken, error) {
	name = strings.TrimSpace(name)
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return IssuedToken{}, ErrManagementForbidden
	}
	if len(name) < 1 || len(name) > 64 {
		return IssuedToken{}, fmt.Errorf("token name must be 1 to 64 characters")
	}
	if maxLinked < 1 {
		return IssuedToken{}, fmt.Errorf("linked-token limit must be positive")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("begin linked-token creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_prefix = token_prefix WHERE id = ''`); err != nil {
		return IssuedToken{}, err
	}
	key := m.currentEncryptionKey()
	if err := secreport.ValidateCurrentEncryptionKey(ctx, tx, key); err != nil {
		return IssuedToken{}, err
	}
	if err := verifyPrimary(ctx, tx, primary); err != nil {
		return IssuedToken{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`, primary.OwnerID, time.Now().Unix()).Scan(&count); err != nil {
		return IssuedToken{}, fmt.Errorf("count active linked tokens: %w", err)
	}
	if count >= maxLinked {
		return IssuedToken{}, ErrLinkedTokenLimit
	}
	var duplicate int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND lower(name) = lower(?)`, primary.OwnerID, name).Scan(&duplicate); err != nil {
		return IssuedToken{}, fmt.Errorf("check linked-token name: %w", err)
	}
	if duplicate > 0 {
		return IssuedToken{}, ErrDuplicateTokenName
	}
	token, err := generateOwnerToken()
	if err != nil {
		return IssuedToken{}, err
	}
	hash, copy, err := m.encryptWithKey(token, key)
	if err != nil {
		return IssuedToken{}, err
	}
	tokenID, err := uuid.NewV7()
	if err != nil {
		return IssuedToken{}, fmt.Errorf("generate linked token id: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO api_tokens
		(id, owner_id, kind, name, token_hash, token_prefix, token_last4, token_enc, expires_at)
		VALUES (?, ?, 'linked', ?, ?, ?, ?, ?, ?)`, tokenID.String(), primary.OwnerID, name, hash, token[:12], token[len(token)-4:], copy, expiresAt)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("store linked token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IssuedToken{}, fmt.Errorf("commit linked-token creation: %w", err)
	}
	return IssuedToken{OwnerID: primary.OwnerID, TokenID: tokenID.String(), Token: token, Prefix: token[:12]}, nil
}

// Reveal returns a token only after primary-owner authorization.
func (m *Manager) Reveal(ctx context.Context, primary Principal, id string) (string, error) {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return "", ErrManagementForbidden
	}
	if err := verifyPrimary(ctx, m.db, primary); err != nil {
		return "", err
	}
	var encrypted string
	if err := m.db.QueryRowContext(ctx, `SELECT token_enc FROM api_tokens WHERE id = ? AND owner_id = ? AND revoked_at IS NULL`, id, primary.OwnerID).Scan(&encrypted); err != nil {
		if err == sql.ErrNoRows {
			return "", ErrInvalidToken
		}
		return "", fmt.Errorf("read token for reveal: %w", err)
	}
	currentKey := m.currentEncryptionKey()
	token, err := DecryptToken(currentKey, encrypted)
	if err != nil {
		previous, valid, previousErr := secret.LoadPreviousEncryptionKey(ctx, m.db, currentKey, time.Now().UTC())
		if previousErr != nil {
			return "", previousErr
		}
		if valid {
			token, usedPrevious, fallbackErr := DecryptTokenWithPrevious(currentKey, previous, encrypted)
			if fallbackErr == nil && usedPrevious {
				tx, txErr := m.db.BeginTx(ctx, nil)
				if txErr != nil {
					return "", txErr
				}
				defer func() { _ = tx.Rollback() }()
				if _, txErr = tx.ExecContext(ctx, `UPDATE api_tokens SET token_prefix = token_prefix WHERE id = ''`); txErr != nil {
					return "", txErr
				}
				if txErr = secreport.ValidateCurrentEncryptionKey(ctx, tx, currentKey); txErr != nil {
					return "", txErr
				}
				rewritten, encErr := EncryptToken(currentKey, token)
				if encErr != nil {
					return "", encErr
				}
				if _, encErr = tx.ExecContext(ctx, `UPDATE api_tokens SET token_enc = ? WHERE id = ?`, rewritten, id); encErr != nil {
					return "", encErr
				}
				if encErr = tx.Commit(); encErr != nil {
					return "", encErr
				}
				err = nil
			} else {
				err = fallbackErr
			}
		}
	}
	if err != nil {
		m.logDecryptionFailure("api_tokens", id)
		return "", fmt.Errorf("decrypt token for reveal: %w", err)
	}
	return token, nil
}

// Rotate replaces a token in place while preserving owner, name, expiry and resource ownership.
func (m *Manager) Rotate(ctx context.Context, primary Principal, id string) (IssuedToken, error) {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return IssuedToken{}, ErrManagementForbidden
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedToken{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_prefix = token_prefix WHERE id = ''`); err != nil {
		return IssuedToken{}, err
	}
	key := m.currentEncryptionKey()
	if err := secreport.ValidateCurrentEncryptionKey(ctx, tx, key); err != nil {
		return IssuedToken{}, err
	}
	if err := verifyPrimary(ctx, tx, primary); err != nil {
		return IssuedToken{}, err
	}
	token, err := generateOwnerToken()
	if err != nil {
		return IssuedToken{}, err
	}
	hash, copy, err := m.encryptWithKey(token, key)
	if err != nil {
		return IssuedToken{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ?, token_prefix = ?, token_last4 = ?, token_enc = ?, rotated_at = ?
		WHERE id = ? AND owner_id = ? AND revoked_at IS NULL AND (kind = 'linked' OR (kind = 'primary' AND id = ?))`,
		hash, token[:12], token[len(token)-4:], copy, time.Now().Unix(), id, primary.OwnerID, primary.TokenID)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("rotate owner token: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return IssuedToken{}, fmt.Errorf("verify token rotation: %w", err)
	}
	if changed != 1 {
		return IssuedToken{}, ErrInvalidToken
	}
	if err := tx.Commit(); err != nil {
		return IssuedToken{}, err
	}
	return IssuedToken{OwnerID: primary.OwnerID, TokenID: id, Token: token, Prefix: token[:12]}, nil
}

// ListLinked returns safe metadata for every linked token, including expired and revoked entries.
func (m *Manager) ListLinked(ctx context.Context, primary Principal) ([]LinkedToken, error) {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return nil, ErrManagementForbidden
	}
	rows, err := m.db.QueryContext(ctx, `SELECT id, COALESCE(name, ''), token_prefix, token_last4, created_at, last_used_at, expires_at, revoked_at
		FROM api_tokens WHERE owner_id = ? AND kind = 'linked' ORDER BY created_at DESC, id DESC`, primary.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("list linked tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var tokens []LinkedToken
	for rows.Next() {
		var item LinkedToken
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, &item.Suffix, &item.CreatedAt, &item.LastUsed, &item.ExpiresAt, &item.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan linked token: %w", err)
		}
		tokens = append(tokens, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read linked tokens: %w", err)
	}
	return tokens, nil
}

// PrimaryMetadata returns masked primary-token details without decrypting its credential.
func (m *Manager) PrimaryMetadata(ctx context.Context, primary Principal) (PrimaryTokenMetadata, error) {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return PrimaryTokenMetadata{}, ErrManagementForbidden
	}
	if err := verifyPrimary(ctx, m.db, primary); err != nil {
		return PrimaryTokenMetadata{}, err
	}
	var item PrimaryTokenMetadata
	err := m.db.QueryRowContext(ctx, `SELECT id, token_prefix, token_last4, created_at, last_used_at
		FROM api_tokens WHERE owner_id = ? AND kind = 'primary' AND revoked_at IS NULL`, primary.OwnerID).
		Scan(&item.ID, &item.Prefix, &item.Suffix, &item.CreatedAt, &item.LastUsed)
	if err == sql.ErrNoRows {
		return PrimaryTokenMetadata{}, ErrInvalidToken
	}
	if err != nil {
		return PrimaryTokenMetadata{}, fmt.Errorf("read primary token metadata: %w", err)
	}
	return item, nil
}

// PurgeIdleOwners removes owner tokens for identities that have no favorites
// and have not used a token within the configured idle period.
func (m *Manager) PurgeIdleOwners(ctx context.Context, idleDays int) (int64, error) {
	if idleDays <= 0 {
		return 0, nil
	}
	days := int64(idleDays)
	const secondsPerDay = int64(24 * time.Hour / time.Second)
	if days > (1<<63-1)/secondsPerDay {
		return 0, fmt.Errorf("idle owner purge period is too large")
	}
	cutoff := time.Now().Unix() - days*secondsPerDay
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin idle owner purge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	const idleOwners = `SELECT o.id FROM owners o WHERE o.id <> 'system' AND o.last_seen_at <= ?
		AND NOT EXISTS (SELECT 1 FROM favorites f WHERE f.owner_id = o.id)`
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_tokens WHERE owner_id IN (`+idleOwners+`)`, cutoff); err != nil {
		return 0, fmt.Errorf("delete idle owner tokens: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM owners WHERE id IN (`+idleOwners+`)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete idle owners: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count purged idle owners: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit idle owner purge: %w", err)
	}
	return removed, nil
}

// RevokeLinked permanently revokes one linked token belonging to the owner.
func (m *Manager) RevokeLinked(ctx context.Context, primary Principal, id string) error {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return ErrManagementForbidden
	}
	if err := verifyPrimary(ctx, m.db, primary); err != nil {
		return err
	}
	result, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ?, revoked_reason = 'owner_revoked'
		WHERE id = ? AND owner_id = ? AND kind = 'linked' AND revoked_at IS NULL`, time.Now().Unix(), id, primary.OwnerID)
	if err != nil {
		return fmt.Errorf("revoke linked token: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify linked-token revocation: %w", err)
	}
	if changed != 1 {
		return ErrInvalidToken
	}
	return nil
}

// UpdateLinked changes a linked token's name and expiry without changing its credential.
func (m *Manager) UpdateLinked(ctx context.Context, primary Principal, id, name string, expiresAt *int64) error {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return ErrManagementForbidden
	}
	if err := verifyPrimary(ctx, m.db, primary); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 64 {
		return fmt.Errorf("token name must be 1 to 64 characters")
	}
	var duplicate int
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ? AND kind = 'linked' AND id <> ? AND lower(name) = lower(?)`, primary.OwnerID, id, name).Scan(&duplicate); err != nil {
		return fmt.Errorf("check linked-token name: %w", err)
	}
	if duplicate > 0 {
		return ErrDuplicateTokenName
	}
	result, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET name = ?, expires_at = ?
		WHERE id = ? AND owner_id = ? AND kind = 'linked' AND revoked_at IS NULL`, name, expiresAt, id, primary.OwnerID)
	if err != nil {
		return fmt.Errorf("update linked token: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify linked-token update: %w", err)
	}
	if changed != 1 {
		return ErrInvalidToken
	}
	return nil
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func verifyPrimary(ctx context.Context, db rowQueryer, primary Principal) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE id = ? AND owner_id = ? AND kind = 'primary' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)`, primary.TokenID, primary.OwnerID, time.Now().Unix()).Scan(&count)
	if err != nil {
		return fmt.Errorf("verify primary token: %w", err)
	}
	if count != 1 {
		return ErrManagementForbidden
	}
	return nil
}

// ErrManagementForbidden is returned when a linked token attempts token management.
var ErrManagementForbidden = fmt.Errorf("token management requires the primary token")

// ErrLinkedTokenLimit indicates the owner already has the configured number of active linked tokens.
var ErrLinkedTokenLimit = fmt.Errorf("active linked-token limit reached")

// ErrDuplicateTokenName indicates a linked token already uses the name.
var ErrDuplicateTokenName = fmt.Errorf("linked-token name already exists")

// ErrInvalidToken intentionally represents missing, revoked, and expired tokens identically.
var ErrInvalidToken = fmt.Errorf("invalid token")

func (m *Manager) encrypt(token string) (string, string, error) {
	return m.encryptWithKey(token, m.currentEncryptionKey())
}

func (m *Manager) encryptWithKey(token, key string) (string, string, error) {
	hash := HashToken(token)
	copy, err := EncryptToken(key, token)
	if err != nil {
		return "", "", err
	}
	return hash, copy, nil
}

func generateOwnerToken() (string, error) {
	var token strings.Builder
	token.Grow(36)
	token.WriteString("own_")
	max := big.NewInt(int64(len(tokenAlphabet)))
	for range 32 {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		token.WriteByte(tokenAlphabet[index.Int64()])
	}
	return token.String(), nil
}

func isOwnerToken(token string) bool {
	if len(token) != 36 || (!strings.HasPrefix(token, "own_") && !strings.HasPrefix(token, "tok_")) {
		return false
	}
	for _, char := range token[4:] {
		if !strings.ContainsRune(tokenAlphabet, char) {
			return false
		}
	}
	return true
}

func randomID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

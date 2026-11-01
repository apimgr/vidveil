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
	"time"
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
	db            *sql.DB
	encryptionKey string
}

// Principal is the owner resolved from a valid token.
type Principal struct {
	OwnerID string
	TokenID int64
	Kind    string
	Prefix  string
}

// IssuedToken contains a raw token only at the issuance boundary.
type IssuedToken struct {
	OwnerID string
	TokenID int64
	Token   string
	Prefix  string
}

// LinkedToken contains safe metadata; raw token values are never included.
type LinkedToken struct {
	ID        int64
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
	ID        int64
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
	token, err := generateToken()
	if err != nil {
		return IssuedToken{}, err
	}
	prefix := token[:12]
	storedHash, storedToken, err := m.encrypt(token)
	if err != nil {
		return IssuedToken{}, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("begin owner token issuance: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO owners(id) VALUES (?)`, ownerID); err != nil {
		return IssuedToken{}, fmt.Errorf("create token owner: %w", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO api_tokens
		(owner_id, kind, name, token_hash, token_prefix, token_suffix, token_enc, resource_type, resource_id)
		VALUES (?, 'primary', NULL, ?, ?, ?, ?, '', '')`, ownerID, storedHash, prefix, token[len(token)-4:], storedToken)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("store primary token: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return IssuedToken{}, fmt.Errorf("read primary token id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IssuedToken{}, fmt.Errorf("commit owner token issuance: %w", err)
	}
	return IssuedToken{OwnerID: ownerID, TokenID: id, Token: token, Prefix: prefix}, nil
}

// Authenticate resolves an active owner token and updates bounded last-use data.
func (m *Manager) Authenticate(ctx context.Context, raw string) (Principal, error) {
	if !strings.HasPrefix(raw, "tok_") {
		return Principal{}, ErrInvalidToken
	}
	want := HashToken(raw)
	rows, err := m.db.QueryContext(ctx, `SELECT id, owner_id, kind, token_hash, token_prefix, revoked_at, expires_at, last_used_at
		FROM api_tokens ORDER BY id`)
	if err != nil {
		return Principal{}, fmt.Errorf("read owner tokens: %w", err)
	}
	type tokenRow struct {
		id                        int64
		owner, kind, hash, prefix string
		revoked, expires, used    sql.NullInt64
	}
	var found *tokenRow
	for rows.Next() {
		var item tokenRow
		if err := rows.Scan(&item.id, &item.owner, &item.kind, &item.hash, &item.prefix, &item.revoked, &item.expires, &item.used); err != nil {
			_ = rows.Close()
			return Principal{}, fmt.Errorf("scan owner token: %w", err)
		}
		storedHash, err := DecryptHash(m.encryptionKey, item.hash)
		if err != nil {
			_ = rows.Close()
			return Principal{}, fmt.Errorf("validate stored owner token %d: %w", item.id, err)
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
	now := time.Now().Unix()
	if found == nil || found.revoked.Valid || (found.expires.Valid && found.expires.Int64 <= now) {
		return Principal{}, ErrInvalidToken
	}
	if !found.used.Valid || found.used.Int64 <= now-60 {
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
	token, err := generateToken()
	if err != nil {
		return IssuedToken{}, err
	}
	hash, copy, err := m.encrypt(token)
	if err != nil {
		return IssuedToken{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO api_tokens
		(owner_id, kind, name, token_hash, token_prefix, token_suffix, token_enc, resource_type, resource_id, expires_at)
		VALUES (?, 'linked', ?, ?, ?, ?, ?, '', '', ?)`, primary.OwnerID, name, hash, token[:12], token[len(token)-4:], copy, expiresAt)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("store linked token: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return IssuedToken{}, fmt.Errorf("read linked token id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IssuedToken{}, fmt.Errorf("commit linked-token creation: %w", err)
	}
	return IssuedToken{OwnerID: primary.OwnerID, TokenID: id, Token: token, Prefix: token[:12]}, nil
}

// Reveal returns a token only after primary-owner authorization.
func (m *Manager) Reveal(ctx context.Context, primary Principal, id int64) (string, error) {
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
	token, err := DecryptToken(m.encryptionKey, encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt token for reveal: %w", err)
	}
	return token, nil
}

// Rotate replaces a token in place while preserving owner, name, expiry and resource ownership.
func (m *Manager) Rotate(ctx context.Context, primary Principal, id int64) (IssuedToken, error) {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return IssuedToken{}, ErrManagementForbidden
	}
	if err := verifyPrimary(ctx, m.db, primary); err != nil {
		return IssuedToken{}, err
	}
	token, err := generateToken()
	if err != nil {
		return IssuedToken{}, err
	}
	hash, copy, err := m.encrypt(token)
	if err != nil {
		return IssuedToken{}, err
	}
	result, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ?, token_prefix = ?, token_suffix = ?, token_enc = ?, rotated_at = ?
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
	return IssuedToken{OwnerID: primary.OwnerID, TokenID: id, Token: token, Prefix: token[:12]}, nil
}

// ListLinked returns safe metadata for every linked token, including expired and revoked entries.
func (m *Manager) ListLinked(ctx context.Context, primary Principal) ([]LinkedToken, error) {
	if primary.Kind != "primary" || primary.OwnerID == "" {
		return nil, ErrManagementForbidden
	}
	rows, err := m.db.QueryContext(ctx, `SELECT id, COALESCE(name, ''), token_prefix, token_suffix, created_at, last_used_at, expires_at, revoked_at
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
	err := m.db.QueryRowContext(ctx, `SELECT id, token_prefix, token_suffix, created_at, last_used_at
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
	const idleOwners = `SELECT o.id FROM owners o WHERE o.last_seen_at <= ?
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
func (m *Manager) RevokeLinked(ctx context.Context, primary Principal, id int64) error {
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
func (m *Manager) UpdateLinked(ctx context.Context, primary Principal, id int64, name string, expiresAt *int64) error {
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
	hash := HashToken(token)
	copy, err := EncryptToken(m.encryptionKey, token)
	if err != nil {
		return "", "", err
	}
	return hash, copy, nil
}

func generateToken() (string, error) {
	var token strings.Builder
	token.Grow(36)
	token.WriteString("tok_")
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

func randomID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

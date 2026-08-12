package pgp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// storeQueryTimeout bounds every keypair-metadata query so a stalled DB cannot
// hang a caller (backend-rules: never run a DB query without a context timeout).
const storeQueryTimeout = 5 * time.Second

// KeypairMeta is the on-disk-independent metadata persisted in pgp_keypair.
type KeypairMeta struct {
	Fingerprint         string
	CreatedAt           time.Time
	ExpiresAt           time.Time
	LastRotatedAt       *time.Time
	KeyserversPublished []KeyserverState
	Revoked             bool
}

// KeyserverState records that the public key was published to a keyserver.
type KeyserverState struct {
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// SaveKeypairMeta replaces the single keypair metadata row with the given
// keypair's details. Any prior row is removed first so the table always
// reflects the current live keypair.
func SaveKeypairMeta(db *sql.DB, kp *Keypair) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeQueryTimeout)
	defer cancel()
	if _, err := db.ExecContext(ctx, `DELETE FROM pgp_keypair`); err != nil {
		return fmt.Errorf("clear keypair meta: %w", err)
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO pgp_keypair (fingerprint, created_at, expires_at, keyservers_published, revoked)
		 VALUES (?, ?, ?, '[]', 0)`,
		kp.Fingerprint, kp.CreatedAt.UTC(), kp.ExpiresAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert keypair meta: %w", err)
	}
	return nil
}

// SetLastRotated stamps the current keypair row's last_rotated_at column,
// recording when the keypair was most recently rotated (AI.md PART 12 keypair
// field "last_rotated_at"). It returns an error if there is no keypair row.
func SetLastRotated(db *sql.DB, t time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeQueryTimeout)
	defer cancel()
	res, err := db.ExecContext(ctx,
		`UPDATE pgp_keypair SET last_rotated_at = ?
		 WHERE id = (SELECT id FROM pgp_keypair ORDER BY id DESC LIMIT 1)`,
		t.UTC(),
	)
	if err != nil {
		return fmt.Errorf("update last_rotated_at: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no keypair row to stamp last_rotated_at")
	}
	return nil
}

// MarkRevoked flags the current keypair row as revoked (AI.md PART 12 keypair
// field "revoked": set if Delete was used — the key file may be gone but the
// fingerprint stays in audit history). It returns an error if no row exists.
func MarkRevoked(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeQueryTimeout)
	defer cancel()
	res, err := db.ExecContext(ctx,
		`UPDATE pgp_keypair SET revoked = 1
		 WHERE id = (SELECT id FROM pgp_keypair ORDER BY id DESC LIMIT 1)`,
	)
	if err != nil {
		return fmt.Errorf("mark keypair revoked: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no keypair row to revoke")
	}
	return nil
}

// GetKeypairMeta returns the current keypair metadata, or (nil, nil) if none.
func GetKeypairMeta(db *sql.DB) (*KeypairMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeQueryTimeout)
	defer cancel()
	row := db.QueryRowContext(ctx,
		`SELECT fingerprint, created_at, expires_at, last_rotated_at, keyservers_published, revoked
		 FROM pgp_keypair ORDER BY id DESC LIMIT 1`,
	)
	var (
		m          KeypairMeta
		rotated    sql.NullTime
		published  string
		revokedInt int
	)
	err := row.Scan(&m.Fingerprint, &m.CreatedAt, &m.ExpiresAt, &rotated, &published, &revokedInt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan keypair meta: %w", err)
	}
	if rotated.Valid {
		m.LastRotatedAt = &rotated.Time
	}
	m.Revoked = revokedInt != 0
	if published != "" {
		if err := json.Unmarshal([]byte(published), &m.KeyserversPublished); err != nil {
			return nil, fmt.Errorf("decode keyservers_published: %w", err)
		}
	}
	return &m, nil
}

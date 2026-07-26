package pgp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

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
	if _, err := db.Exec(`DELETE FROM pgp_keypair`); err != nil {
		return fmt.Errorf("clear keypair meta: %w", err)
	}
	_, err := db.Exec(
		`INSERT INTO pgp_keypair (fingerprint, created_at, expires_at, keyservers_published, revoked)
		 VALUES (?, ?, ?, '[]', 0)`,
		kp.Fingerprint, kp.CreatedAt.UTC(), kp.ExpiresAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert keypair meta: %w", err)
	}
	return nil
}

// GetKeypairMeta returns the current keypair metadata, or (nil, nil) if none.
func GetKeypairMeta(db *sql.DB) (*KeypairMeta, error) {
	row := db.QueryRow(
		`SELECT fingerprint, created_at, expires_at, last_rotated_at, keyservers_published, revoked
		 FROM pgp_keypair ORDER BY id DESC LIMIT 1`,
	)
	var (
		m           KeypairMeta
		rotated     sql.NullTime
		published   string
		revokedInt  int
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

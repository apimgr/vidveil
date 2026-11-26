// SPDX-License-Identifier: MIT
package secreport

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/apimgr/vidveil/src/server/service/pgp"
)

// ReencryptPGPBodies re-encrypts every PGP-backed report inside the caller's
// transaction. The caller installs the new key only after committing.
func ReencryptPGPBodies(ctx context.Context, tx *sql.Tx, oldPrivate, newPublic []byte) error {
	rows, err := tx.QueryContext(ctx, `SELECT tracking_id, body_enc FROM security_reports WHERE body_alg = ?`, string(EncryptionMethodPGP))
	if err != nil {
		return fmt.Errorf("select PGP-encrypted security reports: %w", err)
	}
	type report struct {
		id   string
		body []byte
	}
	var reports []report
	for rows.Next() {
		var item report
		if err := rows.Scan(&item.id, &item.body); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan PGP-encrypted security report: %w", err)
		}
		reports = append(reports, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read PGP-encrypted security reports: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range reports {
		plain, err := DecryptPGPReportBody(oldPrivate, item.body)
		if err != nil {
			return fmt.Errorf("decrypt security report %s during key rotation: %w", item.id, err)
		}
		ciphertext, err := pgp.EncryptMessageToPublicKey(newPublic, plain)
		if err != nil {
			return fmt.Errorf("encrypt security report %s for rotated key: %w", item.id, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE security_reports SET body_enc = ?, updated_at = CURRENT_TIMESTAMP WHERE tracking_id = ?`, ciphertext, item.id); err != nil {
			return fmt.Errorf("update security report %s during key rotation: %w", item.id, err)
		}
	}
	return nil
}

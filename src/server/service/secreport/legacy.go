// SPDX-License-Identifier: MIT
package secreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MigrateLegacySchema upgrades the pre-PART-11 report table without ever
// copying plaintext report/contact data into the replacement table.
func MigrateLegacySchema(ctx context.Context, db *sql.DB, configDir, encryptionKeyHex string, installationSecret []byte) error {
	var hasCanonical int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('security_reports') WHERE name = 'body_enc'`).Scan(&hasCanonical)
	if err != nil {
		return fmt.Errorf("inspect security report schema: %w", err)
	}
	if hasCanonical != 0 {
		return nil
	}
	var tableExists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='security_reports'`).Scan(&tableExists); err != nil {
		return fmt.Errorf("inspect legacy security report table: %w", err)
	}
	if tableExists == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT tracking_id, status, severity, component, endpoint, summary, encrypted_body, encryption_method,
		researcher_email, researcher_gpg_fingerprint, cve_requested, credit_preference, credit_name, disclosed,
		report_token_hash, report_token_last_used_date, maintainer_comments, expected_disclosure_date,
		created_at, updated_at, closed_at FROM security_reports ORDER BY created_at, tracking_id`)
	if err != nil {
		return fmt.Errorf("read legacy security reports: %w", err)
	}
	type legacyReport struct {
		trackingID, status, severity, component, endpoint, summary, algorithm, email, researcherKey string
		body                                                                                        []byte
		cve, disclosed                                                                              int
		creditPref                                                                                  string
		creditName, tokenHash, lastUsed, comment                                                    sql.NullString
		disclosure, closedAt                                                                        sql.NullTime
		createdAt, updatedAt                                                                        time.Time
	}
	var legacy []legacyReport
	for rows.Next() {
		var item legacyReport
		var endpoint, email, researcherKey, creditName, lastUsed, comment sql.NullString
		var disclosure, closedAt sql.NullTime
		if err := rows.Scan(&item.trackingID, &item.status, &item.severity, &item.component, &endpoint, &item.summary, &item.body, &item.algorithm,
			&email, &researcherKey, &item.cve, &item.creditPref, &creditName, &item.disclosed, &item.tokenHash, &lastUsed, &comment,
			&disclosure, &item.createdAt, &item.updatedAt, &closedAt); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy security report: %w", err)
		}
		item.endpoint, item.email, item.researcherKey = endpoint.String, email.String, researcherKey.String
		item.creditName, item.lastUsed, item.comment = creditName, lastUsed, comment
		item.disclosure, item.closedAt = disclosure, closedAt
		legacy = append(legacy, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read legacy security reports: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin security report schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `ALTER TABLE security_reports RENAME TO security_reports_legacy`); err != nil {
		return fmt.Errorf("preserve legacy security report table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_security_reports_status`); err != nil {
		return fmt.Errorf("replace legacy security report index: %w", err)
	}
	for _, ddl := range []string{
		`CREATE TABLE security_reports (tracking_id VARCHAR(32) PRIMARY KEY, id_window BIGINT NOT NULL, severity VARCHAR(16) NOT NULL, component VARCHAR(255) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'received', body_enc BLOB NOT NULL, body_alg VARCHAR(16) NOT NULL, contact_enc BLOB, credit_pref VARCHAR(8) NOT NULL DEFAULT 'no', credit_name VARCHAR(100), researcher_comment TEXT NOT NULL DEFAULT '', disclosure_date DATE, cve_requested BOOLEAN NOT NULL DEFAULT FALSE, token_hash VARCHAR(64) NOT NULL, token_last_used DATE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, closed_at TIMESTAMP)`,
		`CREATE INDEX idx_security_reports_status ON security_reports(status, created_at)`,
		`CREATE TABLE IF NOT EXISTS security_thanks (tracking_id VARCHAR(32) PRIMARY KEY, credit VARCHAR(120) NOT NULL, severity VARCHAR(16) NOT NULL, disclosed_on DATE NOT NULL)`,
	} {
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create canonical security report schema: %w", err)
		}
	}
	for _, item := range legacy {
		method := EncryptionMethod(item.algorithm)
		plain, err := DecryptReportBody(configDir, installationSecret, encryptionKeyHex, method, item.body)
		if err != nil {
			return fmt.Errorf("decrypt legacy report %s: %w", item.trackingID, err)
		}
		parts := make([]string, 0, 3)
		if item.endpoint != "" {
			parts = append(parts, "Endpoint: "+item.endpoint)
		}
		if item.summary != "" {
			parts = append(parts, "Summary: "+item.summary)
		}
		parts = append(parts, string(plain))
		body, bodyAlg, err := EncryptReportBody(configDir, encryptionKeyHex, []byte(strings.Join(parts, "\n\n")))
		if err != nil {
			return fmt.Errorf("reencrypt legacy report %s: %w", item.trackingID, err)
		}
		contactData, err := json.Marshal(ContactDetails{Email: item.email, ResearcherKey: item.researcherKey})
		if err != nil {
			return err
		}
		contact, err := EncryptAESGCM(encryptionKeyHex, contactData)
		if err != nil {
			return fmt.Errorf("encrypt legacy report contact %s: %w", item.trackingID, err)
		}
		pref, name, err := normalizeCredit(item.creditPref, item.creditName.String)
		if err != nil {
			pref, name = "no", nil
		}
		status := item.status
		if !validStatus(status) {
			status = StatusReceived
		}
		if item.disclosed != 0 {
			status = StatusDisclosed
		}
		var disclosure any
		if item.disclosure.Valid {
			disclosure = item.disclosure.Time.UTC().Format("2006-01-02")
		}
		var closed any
		if item.closedAt.Valid {
			closed = item.closedAt.Time.UTC()
		} else if status == StatusDisclosed || status == StatusWontFix {
			closed = item.updatedAt.UTC()
		}
		// The prior schema never stored the validated security_id window; zero
		// marks the historical value as unavailable instead of guessing from
		// report submission time, which can cross a 48-hour boundary.
		window := int64(0)
		tokenHash := item.tokenHash.String
		if !item.tokenHash.Valid || tokenHash == "" {
			tokenHash = strings.Repeat("0", 64)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO security_reports (tracking_id,id_window,severity,component,status,body_enc,body_alg,contact_enc,credit_pref,credit_name,researcher_comment,disclosure_date,cve_requested,token_hash,token_last_used,created_at,updated_at,closed_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			item.trackingID, window, strings.ToLower(item.severity), item.component, status, body, string(bodyAlg), contact, pref, name,
			item.comment.String, disclosure, item.cve, tokenHash, item.lastUsed.String, item.createdAt, item.updatedAt, closed)
		if err != nil {
			return fmt.Errorf("copy legacy security report %s: %w", item.trackingID, err)
		}
		if status == StatusDisclosed && pref != "no" {
			credit := ""
			if pref == "anon" {
				var anonCount int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_thanks WHERE credit LIKE 'Anonymous Researcher #%'`).Scan(&anonCount); err != nil {
					return err
				}
				credit = fmt.Sprintf("Anonymous Researcher #%d", anonCount+1)
			} else if name != nil {
				credit = name.(string)
			}
			if credit != "" {
				disclosedOn := item.updatedAt.UTC().Format("2006-01-02")
				if item.closedAt.Valid {
					disclosedOn = item.closedAt.Time.UTC().Format("2006-01-02")
				}
				if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO security_thanks (tracking_id,credit,severity,disclosed_on) VALUES (?,?,?,?)`, item.trackingID, credit, item.severity, disclosedOn); err != nil {
					return err
				}
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE security_reports_legacy`); err != nil {
		return fmt.Errorf("remove legacy security report table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit security report schema migration: %w", err)
	}
	return nil
}

// SPDX-License-Identifier: MIT
package secreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/dbstore"
)

type legacyReport struct {
	trackingID, status, severity, component, endpoint, summary, algorithm, email, researcherKey string
	body                                                                                        []byte
	cve, disclosed                                                                              int
	creditPref                                                                                  string
	creditName, tokenHash, lastUsed, comment                                                    sql.NullString
	disclosure, closedAt                                                                        sql.NullTime
	createdAt, updatedAt                                                                        time.Time
}

type migratedReport struct {
	trackingID, severity, component, status, algorithm string
	body, contact                                      []byte
	creditPref                                         string
	creditName, comment                                any
	disclosure, lastUsed                               any
	cve                                                int
	tokenHash                                          string
	createdAt, updatedAt                               time.Time
	closedAt                                           any
}

// MigrateLegacySchema upgrades the pre-PART-11 report table without ever
// copying plaintext report/contact data into the replacement table.
func MigrateLegacySchema(ctx context.Context, db Store, configDir, encryptionKeyHex string, installationSecret []byte) error {
	var hasCanonical, tableExists int
	var legacy []legacyReport
	err := db.Read(ctx, func(readCtx context.Context, readDB dbstore.Reader) error {
		if err := readDB.QueryRowContext(readCtx, `SELECT COUNT(*) FROM pragma_table_info('security_reports') WHERE name = 'body_enc'`).Scan(&hasCanonical); err != nil {
			return fmt.Errorf("inspect security report schema: %w", err)
		}
		if hasCanonical != 0 {
			return nil
		}
		if err := readDB.QueryRowContext(readCtx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='security_reports'`).Scan(&tableExists); err != nil {
			return fmt.Errorf("inspect legacy security report table: %w", err)
		}
		if tableExists == 0 {
			return nil
		}
		rows, err := readDB.QueryContext(readCtx, `SELECT tracking_id, status, severity, component, endpoint, summary, encrypted_body, encryption_method,
		researcher_email, researcher_gpg_fingerprint, cve_requested, credit_preference, credit_name, disclosed,
		report_token_hash, report_token_last_used_date, maintainer_comments, expected_disclosure_date,
		created_at, updated_at, closed_at FROM security_reports ORDER BY created_at, tracking_id`)
		if err != nil {
			return fmt.Errorf("read legacy security reports: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var item legacyReport
			var endpoint, email, researcherKey, creditName, lastUsed, comment sql.NullString
			var disclosure, closedAt sql.NullTime
			if err := rows.Scan(&item.trackingID, &item.status, &item.severity, &item.component, &endpoint, &item.summary, &item.body, &item.algorithm,
				&email, &researcherKey, &item.cve, &item.creditPref, &creditName, &item.disclosed, &item.tokenHash, &lastUsed, &comment,
				&disclosure, &item.createdAt, &item.updatedAt, &closedAt); err != nil {
				return fmt.Errorf("scan legacy security report: %w", err)
			}
			item.endpoint, item.email, item.researcherKey = endpoint.String, email.String, researcherKey.String
			item.creditName, item.lastUsed, item.comment = creditName, lastUsed, comment
			item.disclosure, item.closedAt = disclosure, closedAt
			legacy = append(legacy, item)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	if hasCanonical != 0 || tableExists == 0 {
		return nil
	}

	prepared := make([]migratedReport, 0, len(legacy))
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
		tokenHash := item.tokenHash.String
		if !item.tokenHash.Valid || tokenHash == "" {
			tokenHash = strings.Repeat("0", 64)
		}
		prepared = append(prepared, migratedReport{trackingID: item.trackingID, severity: strings.ToLower(item.severity), component: item.component, status: status, algorithm: string(bodyAlg), body: body, contact: contact, creditPref: pref, creditName: name, comment: item.comment.String, disclosure: disclosure, cve: item.cve, tokenHash: tokenHash, lastUsed: item.lastUsed.String, createdAt: item.createdAt, updatedAt: item.updatedAt, closedAt: closed})
	}
	return db.Write(ctx, func(tx *sql.Tx) error {
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
		for _, item := range prepared {
			// The prior schema never stored the validated security_id window; zero
			// marks the historical value as unavailable instead of guessing from
			// report submission time, which can cross a 48-hour boundary.
			const window int64 = 0
			_, err := tx.ExecContext(ctx, `INSERT INTO security_reports (tracking_id,id_window,severity,component,status,body_enc,body_alg,contact_enc,credit_pref,credit_name,researcher_comment,disclosure_date,cve_requested,token_hash,token_last_used,created_at,updated_at,closed_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				item.trackingID, window, item.severity, item.component, item.status, item.body, item.algorithm, item.contact, item.creditPref, item.creditName,
				item.comment, item.disclosure, item.cve, item.tokenHash, item.lastUsed, item.createdAt, item.updatedAt, item.closedAt)
			if err != nil {
				return fmt.Errorf("copy legacy security report %s: %w", item.trackingID, err)
			}
			if item.status == StatusDisclosed && item.creditPref != "no" {
				credit := ""
				if item.creditPref == "anon" {
					var anonCount int
					if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_thanks WHERE credit LIKE 'Anonymous Researcher #%'`).Scan(&anonCount); err != nil {
						return err
					}
					credit = fmt.Sprintf("Anonymous Researcher #%d", anonCount+1)
				} else if name, ok := item.creditName.(string); ok {
					credit = name
				}
				if credit != "" {
					disclosedOn := item.updatedAt.UTC().Format("2006-01-02")
					if closedAt, ok := item.closedAt.(time.Time); ok {
						disclosedOn = closedAt.UTC().Format("2006-01-02")
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
		return nil
	})
}

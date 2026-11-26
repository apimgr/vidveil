// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — Coordinated Disclosure Pipeline
// Tracking-metadata store for the security_reports table. The report body
// is encrypted at rest and plaintext is never persisted; there is no admin
// web UI/API for this table (AI.md PART 11 "Security Administration") —
// status/comments are set by out-of-band maintainer tooling.
package secreport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Status values for security_reports.status, per AI.md PART 11 "Public
// Pages" researcher status page states.
const (
	StatusReceived  = "received"
	StatusTriaged   = "triaged"
	StatusConfirmed = "confirmed"
	StatusPatching  = "patching"
	StatusDisclosed = "disclosed"
	StatusWontFix   = "wont_fix"
)

// reportTokenGrace is how long a one-shot report status token remains valid
// after the report is closed (AI.md PART 11: "expires after the report is
// closed for 30 days").
const reportTokenGrace = 30 * 24 * time.Hour

var rotationMu sync.RWMutex

// LockKeyRotation pauses report submissions until the key rotation commit and
// staged-file install finish. The returned function releases the lock.
func LockKeyRotation() func() {
	rotationMu.Lock()
	return rotationMu.Unlock
}

// Input carries the security-mode contact form fields for a new report.
type Input struct {
	IDWindow  int64
	Severity  string
	Component string
	// Body contains only vulnerability details. Contact details are encrypted
	// independently in contact_enc.
	Body                     []byte
	ResearcherEmail          string
	ResearcherGPGFingerprint string
	CVERequested             bool
	DisclosureWindowDays     int
	CreditPreference         string
	CreditName               string
	AppVersion               string
	CommitHash               string
}

// ContactDetails contains the sensitive researcher contact data. It is stored
// only as AES-256-GCM ciphertext in security_reports.contact_enc.
type ContactDetails struct {
	Email         string `json:"email,omitempty"`
	ResearcherKey string `json:"researcher_key,omitempty"`
}

// ThanksEntry is the public, contact-free hall-of-fame row.
type ThanksEntry struct {
	Credit      string `json:"credit"`
	Severity    string `json:"severity"`
	DisclosedOn string `json:"date"`
}

// ReportSummary is safe for operator list output and contains no report or
// contact content.
type ReportSummary struct {
	TrackingID string
	Status     string
	Severity   string
	Component  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ReportDetails contains only metadata and the researcher's non-sensitive
// status comment. It never includes report body or contact ciphertext.
type ReportDetails struct {
	ReportSummary
	ResearcherComment string
	DisclosureDate    sql.NullTime
}

// UpdateInput describes an operator-authenticated maintenance update.
type UpdateInput struct {
	Status         *string
	Comment        *string
	DisclosureDate *time.Time
	Severity       *string
}

// UpdateResult contains the old and new public status for audit logging.
type UpdateResult struct {
	OldStatus string
	NewStatus string
}

// Submission is returned to the caller after a report is stored: the
// tracking id (safe to return/email) and the one-shot report token
// (returned ONLY here and in the researcher acknowledgment email; only its
// SHA-256 hash is persisted).
type Submission struct {
	TrackingID    string
	ReportToken   string
	EncryptedBody []byte
	BodyAlgorithm EncryptionMethod
}

// CreateReport encrypts in.Body at rest and inserts a new security_reports
// row. Per AI.md PART 11 step 7, this function does not log report content;
// callers are responsible for the security.log security.report_received
// entry (tracking_id, severity, sanitized component — no PII/content).
func CreateReport(ctx context.Context, db *sql.DB, configDir, encryptionKeyHex string, in Input) (*Submission, error) {
	rotationMu.RLock()
	defer rotationMu.RUnlock()
	if in.IDWindow == 0 {
		return nil, fmt.Errorf("missing validated security-id window")
	}
	if !validSeverity(in.Severity) {
		return nil, fmt.Errorf("invalid report severity")
	}
	if strings.TrimSpace(in.Component) == "" || len(in.Component) > 255 || len(in.Body) == 0 {
		return nil, fmt.Errorf("invalid report component or body")
	}
	creditPref, creditName, err := normalizeCredit(in.CreditPreference, in.CreditName)
	if err != nil {
		return nil, err
	}
	trackingID, err := newTrackingID()
	if err != nil {
		return nil, fmt.Errorf("generate tracking id: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin security report submission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Acquire the SQLite writer lock before reading the active PGP key or
	// encrypting the body, so cross-process key rotation cannot miss this row.
	if _, err := tx.ExecContext(ctx, `UPDATE security_reports SET updated_at = updated_at WHERE tracking_id = ?`, trackingID); err != nil {
		return nil, fmt.Errorf("lock security report submission: %w", err)
	}
	if err := ValidateCurrentEncryptionKey(ctx, tx, encryptionKeyHex); err != nil {
		return nil, fmt.Errorf("reject security report with stale encryption key: %w", err)
	}

	encryptedBody, method, err := EncryptReportBody(configDir, encryptionKeyHex, in.Body)
	if err != nil {
		return nil, fmt.Errorf("encrypt report body: %w", err)
	}

	reportToken, tokenHash, err := newReportToken()
	if err != nil {
		return nil, fmt.Errorf("generate report token: %w", err)
	}
	contact, err := json.Marshal(ContactDetails{Email: in.ResearcherEmail, ResearcherKey: in.ResearcherGPGFingerprint})
	if err != nil {
		return nil, fmt.Errorf("encode researcher contact details: %w", err)
	}
	contactEnc, err := EncryptAESGCM(encryptionKeyHex, contact)
	if err != nil {
		return nil, fmt.Errorf("encrypt researcher contact details: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO security_reports (
			tracking_id, id_window, severity, component, status, body_enc, body_alg,
			contact_enc, credit_pref, credit_name, cve_requested, token_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackingID, in.IDWindow, strings.ToLower(in.Severity), strings.TrimSpace(in.Component), StatusReceived,
		encryptedBody, string(method), contactEnc, creditPref, creditName,
		boolToInt(in.CVERequested), tokenHash)
	if err != nil {
		return nil, fmt.Errorf("insert security report: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit security report submission: %w", err)
	}

	return &Submission{TrackingID: trackingID, ReportToken: reportToken, EncryptedBody: append([]byte(nil), encryptedBody...), BodyAlgorithm: method}, nil
}

// Status is the researcher-facing view of a report, per AI.md PART 11
// "/server/security/report/{tracking_id}".
type Status struct {
	TrackingID             string
	Status                 string
	MaintainerComments     string
	ExpectedDisclosureDate sql.NullTime
	CreatedAt              time.Time
}

// GetReportStatus validates the one-shot token (constant-time, hashed
// comparison) and returns the researcher-facing status. The token is
// single-use-per-day: repeat lookups on the same calendar date are allowed,
// but the "last used" date is recorded so a distinct researcher UI/tool can
// rate-limit; the token itself always expires 30 days after closed_at.
func GetReportStatus(ctx context.Context, db *sql.DB, trackingID, token string) (*Status, error) {
	var (
		status, tokenHash  string
		maintainerComments sql.NullString
		createdAt          time.Time
		closedAt           sql.NullTime
		expectedDisclosure sql.NullTime
		lastUsedDate       sql.NullString
	)
	err := db.QueryRowContext(ctx, `
		SELECT status, token_hash, researcher_comment, created_at,
	       closed_at, disclosure_date, token_last_used
		FROM security_reports WHERE tracking_id = ?`, trackingID).
		Scan(&status, &tokenHash, &maintainerComments, &createdAt, &closedAt, &expectedDisclosure, &lastUsedDate)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("report not found")
		}
		return nil, fmt.Errorf("get report status: %w", err)
	}

	if !validTokenHash(tokenHash, token) {
		return nil, fmt.Errorf("invalid report token")
	}
	if closedAt.Valid && time.Now().After(closedAt.Time.Add(reportTokenGrace)) {
		return nil, fmt.Errorf("report token expired")
	}
	usedDate := time.Now().UTC().Format("2006-01-02")
	if lastUsedDate.Valid && lastUsedDate.String == usedDate {
		return nil, fmt.Errorf("report token already used today")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE security_reports SET token_last_used = ?
		WHERE tracking_id = ? AND (token_last_used IS NULL OR token_last_used <> ?)`,
		usedDate, trackingID, usedDate)
	if err != nil {
		return nil, fmt.Errorf("record report status token use: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check report status token use: %w", err)
	}
	if updated != 1 {
		return nil, fmt.Errorf("report token already used today")
	}

	return &Status{
		TrackingID:             trackingID,
		Status:                 status,
		MaintainerComments:     maintainerComments.String,
		ExpectedDisclosureDate: expectedDisclosure,
		CreatedAt:              createdAt,
	}, nil
}

// ListThanks returns the public hall-of-fame entries newest first. The table
// deliberately contains no contact data and is never affected by report cleanup.
func ListThanks(ctx context.Context, db *sql.DB) ([]ThanksEntry, error) {
	rows, err := db.QueryContext(ctx, `SELECT credit, severity, disclosed_on FROM security_thanks ORDER BY disclosed_on DESC, tracking_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query security thanks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	entries := make([]ThanksEntry, 0)
	for rows.Next() {
		var entry ThanksEntry
		var disclosedOn sql.NullTime
		if err := rows.Scan(&entry.Credit, &entry.Severity, &disclosedOn); err != nil {
			return nil, fmt.Errorf("scan security thanks entry: %w", err)
		}
		if disclosedOn.Valid {
			entry.DisclosedOn = disclosedOn.Time.UTC().Format("2006-01-02")
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read security thanks: %w", err)
	}
	return entries, nil
}

// ListReports returns report metadata only. Encrypted bodies and contact
// ciphertext are never selected by this maintenance listing.
func ListReports(ctx context.Context, db *sql.DB, status string) ([]ReportSummary, error) {
	query := `SELECT tracking_id, status, severity, component, created_at, updated_at FROM security_reports`
	args := []any{}
	if status != "" {
		if !validStatus(status) {
			return nil, fmt.Errorf("invalid security report status %q", status)
		}
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC, tracking_id`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query security reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	reports := make([]ReportSummary, 0)
	for rows.Next() {
		var report ReportSummary
		if err := rows.Scan(&report.TrackingID, &report.Status, &report.Severity, &report.Component, &report.CreatedAt, &report.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan security report summary: %w", err)
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read security reports: %w", err)
	}
	return reports, nil
}

// GetReportDetails returns operator-visible metadata and the researcher
// comment, never report content or private contact details.
func GetReportDetails(ctx context.Context, db *sql.DB, trackingID string) (*ReportDetails, error) {
	var report ReportDetails
	err := db.QueryRowContext(ctx, `
		SELECT tracking_id, status, severity, component, created_at, updated_at,
		       researcher_comment, disclosure_date
		FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(
		&report.TrackingID, &report.Status, &report.Severity, &report.Component,
		&report.CreatedAt, &report.UpdatedAt, &report.ResearcherComment, &report.DisclosureDate)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("security report %q not found", trackingID)
		}
		return nil, fmt.Errorf("get security report metadata: %w", err)
	}
	return &report, nil
}

// EncryptedBody returns the original stored ciphertext and algorithm label for
// an encrypted notification export. It never decrypts report content.
func EncryptedBody(ctx context.Context, db *sql.DB, trackingID string) ([]byte, EncryptionMethod, error) {
	return EncryptedBodyWithKeys(ctx, db, trackingID, "", "")
}

// EncryptedBodyWithKeys opportunistically migrates an AES ciphertext still
// encrypted by the grace-period key, then returns only encrypted bytes.
func EncryptedBodyWithKeys(ctx context.Context, db *sql.DB, trackingID, currentKey, previousKey string) ([]byte, EncryptionMethod, error) {
	var body []byte
	var method string
	if err := db.QueryRowContext(ctx, `SELECT body_enc, body_alg FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&body, &method); err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("security report %q not found", trackingID)
		}
		return nil, "", fmt.Errorf("read encrypted security report: %w", err)
	}
	if method != string(EncryptionMethodPGP) && method != string(EncryptionMethodAES) {
		return nil, "", fmt.Errorf("unsupported report body algorithm %q", method)
	}
	originalBody := append([]byte(nil), body...)
	if method == string(EncryptionMethodAES) && currentKey != "" {
		_, err := DecryptAESGCM(currentKey, body)
		if err != nil {
			if previousKey == "" {
				return nil, "", fmt.Errorf("decrypt security report %s with current key: %w", trackingID, err)
			}
			plain, err := DecryptAESGCM(previousKey, body)
			if err != nil {
				return nil, "", fmt.Errorf("decrypt security report %s with current or previous key: %w", trackingID, err)
			}
			body, err = EncryptAESGCM(currentKey, plain)
			if err != nil {
				return nil, "", err
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return nil, "", fmt.Errorf("begin report body rewrite: %w", err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err := tx.ExecContext(ctx, `UPDATE security_reports SET updated_at = updated_at WHERE tracking_id = ?`, trackingID); err != nil {
				return nil, "", fmt.Errorf("lock security report body rewrite: %w", err)
			}
			if err := ValidateCurrentEncryptionKey(ctx, tx, currentKey); err != nil {
				return nil, "", fmt.Errorf("reject security report body rewrite with stale key: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE security_reports SET body_enc = ?, updated_at = CURRENT_TIMESTAMP WHERE tracking_id = ? AND body_enc = ?`, body, trackingID, originalBody); err != nil {
				return nil, "", fmt.Errorf("store reencrypted security report %s: %w", trackingID, err)
			}
			if err := tx.Commit(); err != nil {
				return nil, "", fmt.Errorf("commit security report body rewrite: %w", err)
			}
		}
	}
	return body, EncryptionMethod(method), nil
}

// GetContact decrypts only the contact envelope needed for a researcher status
// notice. It never decrypts or returns the vulnerability report body.
func GetContact(ctx context.Context, db *sql.DB, trackingID, encryptionKeyHex string) (*ContactDetails, error) {
	return GetContactWithPrevious(ctx, db, trackingID, encryptionKeyHex, "")
}

// GetContactWithPrevious tries the current key first, then the optional grace
// key. A successful fallback is immediately rewritten under the current key.
func GetContactWithPrevious(ctx context.Context, db *sql.DB, trackingID, encryptionKeyHex, previousKeyHex string) (*ContactDetails, error) {
	var encrypted []byte
	if err := db.QueryRowContext(ctx, `SELECT contact_enc FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&encrypted); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("security report %q not found", trackingID)
		}
		return nil, fmt.Errorf("read encrypted security contact: %w", err)
	}
	if len(encrypted) == 0 {
		return nil, fmt.Errorf("security report %q has no contact details", trackingID)
	}
	plain, err := DecryptAESGCM(encryptionKeyHex, encrypted)
	if err != nil && previousKeyHex != "" {
		plain, err = DecryptAESGCM(previousKeyHex, encrypted)
		if err == nil {
			rewritten, encErr := EncryptAESGCM(encryptionKeyHex, plain)
			if encErr != nil {
				return nil, fmt.Errorf("reencrypt security contact: %w", encErr)
			}
			tx, txErr := db.BeginTx(ctx, nil)
			if txErr != nil {
				return nil, fmt.Errorf("begin security contact rewrite: %w", txErr)
			}
			defer func() { _ = tx.Rollback() }()
			if _, txErr = tx.ExecContext(ctx, `UPDATE security_reports SET updated_at = updated_at WHERE tracking_id = ?`, trackingID); txErr != nil {
				return nil, fmt.Errorf("lock security contact rewrite: %w", txErr)
			}
			if txErr = ValidateCurrentEncryptionKey(ctx, tx, encryptionKeyHex); txErr != nil {
				return nil, fmt.Errorf("reject security contact rewrite with stale key: %w", txErr)
			}
			if _, txErr = tx.ExecContext(ctx, `UPDATE security_reports SET contact_enc = ?, updated_at = CURRENT_TIMESTAMP WHERE tracking_id = ? AND contact_enc = ?`, rewritten, trackingID, encrypted); txErr != nil {
				return nil, fmt.Errorf("store reencrypted security contact: %w", txErr)
			}
			if txErr = tx.Commit(); txErr != nil {
				return nil, fmt.Errorf("commit security contact rewrite: %w", txErr)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decrypt security contact: %w", err)
	}
	var contact ContactDetails
	if err := json.Unmarshal(plain, &contact); err != nil {
		return nil, fmt.Errorf("decode security contact: %w", err)
	}
	return &contact, nil
}

// UpdateReport applies a forward-only operator update and writes the public
// hall-of-fame row atomically with a first transition to disclosed.
func UpdateReport(ctx context.Context, db *sql.DB, trackingID string, update UpdateInput, now time.Time) (*UpdateResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin security report update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var oldStatus, severity, creditPref string
	var creditName sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT status, severity, credit_pref, credit_name FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&oldStatus, &severity, &creditPref, &creditName); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("security report %q not found", trackingID)
		}
		return nil, fmt.Errorf("read security report for update: %w", err)
	}
	newStatus := oldStatus
	if update.Status != nil {
		newStatus = strings.ToLower(strings.TrimSpace(*update.Status))
		if !validStatus(newStatus) || !statusMayAdvance(oldStatus, newStatus) {
			return nil, fmt.Errorf("security report status cannot move from %q to %q", oldStatus, newStatus)
		}
	}
	if update.Severity != nil {
		severity = strings.ToLower(strings.TrimSpace(*update.Severity))
		if !validSeverity(severity) {
			return nil, fmt.Errorf("invalid security report severity")
		}
	}
	comment := ""
	if update.Comment != nil {
		comment = strings.TrimSpace(*update.Comment)
		if len(comment) > 2000 {
			return nil, fmt.Errorf("researcher comment exceeds 2000 characters")
		}
	} else if err := tx.QueryRowContext(ctx, `SELECT researcher_comment FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&comment); err != nil {
		return nil, fmt.Errorf("read security report comment: %w", err)
	}
	var disclosureDate any
	if update.DisclosureDate != nil {
		disclosureDate = update.DisclosureDate.UTC().Format("2006-01-02")
	} else {
		var current sql.NullTime
		if err := tx.QueryRowContext(ctx, `SELECT disclosure_date FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&current); err != nil {
			return nil, fmt.Errorf("read security report disclosure date: %w", err)
		}
		if current.Valid {
			disclosureDate = current.Time.UTC().Format("2006-01-02")
		}
	}
	var closedAt any
	if newStatus == StatusDisclosed || newStatus == StatusWontFix {
		closedAt = now.UTC()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE security_reports SET status = ?, severity = ?, researcher_comment = ?, disclosure_date = ?, closed_at = COALESCE(closed_at, ?), updated_at = ? WHERE tracking_id = ?`,
		newStatus, severity, comment, disclosureDate, closedAt, now.UTC(), trackingID); err != nil {
		return nil, fmt.Errorf("update security report: %w", err)
	}
	if oldStatus != StatusDisclosed && newStatus == StatusDisclosed && creditPref != "no" {
		credit := ""
		if creditPref == "anon" {
			var anonymousCount int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_thanks WHERE credit LIKE 'Anonymous Researcher #%'`).Scan(&anonymousCount); err != nil {
				return nil, fmt.Errorf("count anonymous security thanks entries: %w", err)
			}
			credit = fmt.Sprintf("Anonymous Researcher #%d", anonymousCount+1)
		} else if creditPref == "name" || creditPref == "handle" {
			credit = creditName.String
			if !creditName.Valid || strings.TrimSpace(credit) == "" {
				return nil, fmt.Errorf("security report credit name is missing")
			}
		} else {
			return nil, fmt.Errorf("invalid security report credit preference %q", creditPref)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO security_thanks (tracking_id, credit, severity, disclosed_on) VALUES (?, ?, ?, ?)`, trackingID, credit, severity, now.UTC().Format("2006-01-02")); err != nil {
			return nil, fmt.Errorf("add security thanks entry: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit security report update: %w", err)
	}
	return &UpdateResult{OldStatus: oldStatus, NewStatus: newStatus}, nil
}

// CleanupClosedReports deletes only reports whose closed_at has passed the
// configured retention interval. security_thanks is intentionally retained.
func CleanupClosedReports(ctx context.Context, db *sql.DB, retentionDays int, now time.Time) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 365
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	result, err := db.ExecContext(ctx, `DELETE FROM security_reports WHERE closed_at IS NOT NULL AND closed_at <= ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete expired security reports: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted security reports: %w", err)
	}
	return count, nil
}

func statusMayAdvance(oldStatus, newStatus string) bool {
	if oldStatus == newStatus {
		return true
	}
	if oldStatus == StatusDisclosed || oldStatus == StatusWontFix {
		return false
	}
	if newStatus == StatusWontFix {
		return true
	}
	rank := map[string]int{StatusReceived: 0, StatusTriaged: 1, StatusConfirmed: 2, StatusPatching: 3, StatusDisclosed: 4}
	return rank[newStatus] > rank[oldStatus]
}

func validStatus(status string) bool {
	return status == StatusReceived || status == StatusTriaged || status == StatusConfirmed || status == StatusPatching || status == StatusDisclosed || status == StatusWontFix
}

func validSeverity(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "high", "medium", "low", "informational":
		return true
	default:
		return false
	}
}

func normalizeCredit(preference, name string) (string, any, error) {
	switch strings.ToLower(strings.TrimSpace(preference)) {
	case "name", "real_name":
		if strings.TrimSpace(name) == "" || len(name) > 100 {
			return "", nil, fmt.Errorf("invalid security report credit name")
		}
		return "name", strings.TrimSpace(name), nil
	case "handle":
		if strings.TrimSpace(name) == "" || len(name) > 100 {
			return "", nil, fmt.Errorf("invalid security report credit handle")
		}
		return "handle", strings.TrimSpace(name), nil
	case "anon", "anonymous":
		return "anon", nil, nil
	case "no", "none":
		return "no", nil, nil
	default:
		return "", nil, fmt.Errorf("invalid security report credit preference")
	}
}

func validTokenHash(storedHash, suppliedToken string) bool {
	sum := sha256.Sum256([]byte(suppliedToken))
	expectedHash := sha256.Sum256([]byte(hex.EncodeToString(sum[:])))
	storedHashDigest := sha256.Sum256([]byte(storedHash))
	return subtle.ConstantTimeCompare(storedHashDigest[:], expectedHash[:]) == 1
}

func newTrackingID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sec_" + hex.EncodeToString(b), nil
}

func newReportToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

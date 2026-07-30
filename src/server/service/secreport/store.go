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
	"fmt"
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

// Input carries the security-mode contact form fields for a new report.
type Input struct {
	Severity                 string
	Component                string
	Endpoint                 string
	Summary                  string
	Body                     []byte // plaintext: steps to reproduce + impact + suggested fix, encrypted before storage
	ResearcherEmail          string
	ResearcherGPGFingerprint string
	CVERequested             bool
	DisclosureWindowDays     int
	CreditPreference         string
	CreditName               string
	AppVersion               string
	CommitHash               string
}

// Submission is returned to the caller after a report is stored: the
// tracking id (safe to return/email) and the one-shot report token
// (returned ONLY here and in the researcher acknowledgment email; only its
// SHA-256 hash is persisted).
type Submission struct {
	TrackingID  string
	ReportToken string
}

// CreateReport encrypts in.Body at rest and inserts a new security_reports
// row. Per AI.md PART 11 step 7, this function does not log report content;
// callers are responsible for the security.log security.report_received
// entry (tracking_id, severity, sanitized component — no PII/content).
func CreateReport(ctx context.Context, db *sql.DB, configDir, encryptionKeyHex string, in Input) (*Submission, error) {
	trackingID, err := newTrackingID()
	if err != nil {
		return nil, fmt.Errorf("generate tracking id: %w", err)
	}

	encryptedBody, method, err := EncryptReportBody(configDir, encryptionKeyHex, in.Body)
	if err != nil {
		return nil, fmt.Errorf("encrypt report body: %w", err)
	}

	reportToken, tokenHash, err := newReportToken()
	if err != nil {
		return nil, fmt.Errorf("generate report token: %w", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO security_reports (
			tracking_id, status, severity, component, endpoint, summary,
			encrypted_body, encryption_method, researcher_email,
			researcher_gpg_fingerprint, cve_requested, disclosure_window_days,
			credit_preference, credit_name, app_version, commit_hash,
			report_token_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackingID, StatusReceived, in.Severity, in.Component, in.Endpoint, in.Summary,
		encryptedBody, string(method), in.ResearcherEmail,
		in.ResearcherGPGFingerprint, boolToInt(in.CVERequested), in.DisclosureWindowDays,
		in.CreditPreference, in.CreditName, in.AppVersion, in.CommitHash,
		tokenHash, time.Now(), time.Now())
	if err != nil {
		return nil, fmt.Errorf("insert security report: %w", err)
	}

	return &Submission{TrackingID: trackingID, ReportToken: reportToken}, nil
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
	)
	err := db.QueryRowContext(ctx, `
		SELECT status, report_token_hash, maintainer_comments, created_at,
		       closed_at, expected_disclosure_date
		FROM security_reports WHERE tracking_id = ?`, trackingID).
		Scan(&status, &tokenHash, &maintainerComments, &createdAt, &closedAt, &expectedDisclosure)
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

	_, _ = db.ExecContext(ctx,
		`UPDATE security_reports SET report_token_last_used_date = ? WHERE tracking_id = ?`,
		time.Now().Format("2006-01-02"), trackingID)

	return &Status{
		TrackingID:             trackingID,
		Status:                 status,
		MaintainerComments:     maintainerComments.String,
		ExpectedDisclosureDate: expectedDisclosure,
		CreatedAt:              createdAt,
	}, nil
}

func validTokenHash(storedHash, suppliedToken string) bool {
	if storedHash == "" || suppliedToken == "" {
		return false
	}
	sum := sha256.Sum256([]byte(suppliedToken))
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(hex.EncodeToString(sum[:]))) == 1
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

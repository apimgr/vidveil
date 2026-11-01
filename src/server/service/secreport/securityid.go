// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — Coordinated Disclosure Pipeline
// Implements the rotating one-shot {security_id} token used to gate the
// /server/contact?security_id={id} security-report mode and the
// security.txt Contact: line.
package secreport

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"time"
)

// windowSeconds is the {security_id} rotation period: 48 hours.
const windowSeconds = 172800

// GenerateSecurityID computes the current {security_id} for t.
// Algorithm (AI.md PART 11): HMAC-SHA256(installation_secret,
// floor(unix_seconds / 172800)) -> hex-encode -> first 16 chars.
func GenerateSecurityID(installationSecret []byte, t time.Time) string {
	return computeSecurityID(installationSecret, windowIndex(t))
}

// ValidateSecurityID accepts the current AND previous 48h window's id, per
// AI.md PART 11 "prevents boundary failures for researchers who load the
// security.txt at second 47:59:59".
func ValidateSecurityID(installationSecret []byte, id string, t time.Time) bool {
	current := windowIndex(t)
	idHash := sha256.Sum256([]byte(id))
	currentIDHash := sha256.Sum256([]byte(computeSecurityID(installationSecret, current)))
	previousIDHash := sha256.Sum256([]byte(computeSecurityID(installationSecret, current-1)))
	valid := subtle.ConstantTimeCompare(idHash[:], currentIDHash[:])
	valid |= subtle.ConstantTimeCompare(idHash[:], previousIDHash[:])
	return valid == 1
}

func windowIndex(t time.Time) int64 {
	return t.Unix() / windowSeconds
}

func computeSecurityID(installationSecret []byte, window int64) string {
	mac := hmac.New(sha256.New, installationSecret)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(window))
	mac.Write(buf)
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

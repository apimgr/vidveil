// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — unit tests for the rotating {security_id}.
package secreport

import (
	"testing"
	"time"
)

func TestGenerateAndValidateSecurityID(t *testing.T) {
	secret := []byte("test-installation-secret")
	now := time.Unix(1_700_000_000, 0)

	id := GenerateSecurityID(secret, now)
	if len(id) != 16 {
		t.Fatalf("GenerateSecurityID: want 16 chars, got %d (%q)", len(id), id)
	}
	if !ValidateSecurityID(secret, id, now) {
		t.Fatalf("ValidateSecurityID: expected id %q to validate at generation time", id)
	}
}

func TestValidateSecurityID_PreviousWindowAccepted(t *testing.T) {
	secret := []byte("test-installation-secret")
	now := time.Unix(1_700_000_000, 0)
	id := GenerateSecurityID(secret, now)

	// One window later (48h) the id should still validate against the
	// "previous window" grace per AI.md PART 11: "prevents boundary failures".
	later := now.Add(windowSeconds * time.Second)
	if !ValidateSecurityID(secret, id, later) {
		t.Fatalf("ValidateSecurityID: expected previous-window id %q to still validate", id)
	}
}

func TestValidateSecurityID_ExpiredWindowRejected(t *testing.T) {
	secret := []byte("test-installation-secret")
	now := time.Unix(1_700_000_000, 0)
	id := GenerateSecurityID(secret, now)

	// Two windows later, neither current nor previous should match.
	tooLate := now.Add(2 * windowSeconds * time.Second)
	if ValidateSecurityID(secret, id, tooLate) {
		t.Fatalf("ValidateSecurityID: expected id %q to be rejected two windows later", id)
	}
}

func TestValidateSecurityID_RejectsInvalidInput(t *testing.T) {
	secret := []byte("test-installation-secret")
	now := time.Unix(1_700_000_000, 0)

	if ValidateSecurityID(secret, "", now) {
		t.Fatalf("ValidateSecurityID: expected empty id to be rejected")
	}
	if ValidateSecurityID(secret, "tooshort", now) {
		t.Fatalf("ValidateSecurityID: expected wrong-length id to be rejected")
	}
	if ValidateSecurityID(secret, "0123456789abcdef", now) {
		t.Fatalf("ValidateSecurityID: expected an unrelated 16-char id to be rejected")
	}
}

func TestValidateSecurityID_DifferentSecretRejected(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	id := GenerateSecurityID([]byte("secret-a"), now)
	if ValidateSecurityID([]byte("secret-b"), id, now) {
		t.Fatalf("ValidateSecurityID: expected id generated with a different secret to be rejected")
	}
}

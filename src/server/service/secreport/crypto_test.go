// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — unit tests for at-rest AES-256-GCM encryption.
package secreport

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func testKeyHex(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return hex.EncodeToString(key)
}

func TestEncryptDecryptAESGCM_RoundTrip(t *testing.T) {
	keyHex := testKeyHex(t)
	plaintext := []byte("steps to reproduce: do the thing; impact: bad things happen")

	ciphertext, err := EncryptAESGCM(keyHex, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}
	if string(ciphertext) == string(plaintext) {
		t.Fatalf("EncryptAESGCM: ciphertext must not equal plaintext")
	}

	decrypted, err := DecryptAESGCM(keyHex, ciphertext)
	if err != nil {
		t.Fatalf("DecryptAESGCM: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("DecryptAESGCM: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptAESGCM_WrongKeyFails(t *testing.T) {
	ciphertext, err := EncryptAESGCM(testKeyHex(t), []byte("secret report body"))
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}
	if _, err := DecryptAESGCM(testKeyHex(t), ciphertext); err == nil {
		t.Fatalf("DecryptAESGCM: expected error decrypting with a different key")
	}
}

func TestEncryptAESGCM_InvalidHexKey(t *testing.T) {
	if _, err := EncryptAESGCM("not-hex", []byte("body")); err == nil {
		t.Fatalf("EncryptAESGCM: expected error for non-hex key")
	}
}

func TestEncryptReportBody_FallsBackToAESWithoutPGPKey(t *testing.T) {
	keyHex := testKeyHex(t)
	// configDir with no pgp keypair present -> falls back to AES-GCM per
	// AI.md PART 11 Submission Flow step 3.
	encrypted, method, err := EncryptReportBody(t.TempDir(), keyHex, []byte("report body"))
	if err != nil {
		t.Fatalf("EncryptReportBody: %v", err)
	}
	if method != EncryptionMethodAES {
		t.Fatalf("EncryptReportBody: want method %q, got %q", EncryptionMethodAES, method)
	}
	decrypted, err := DecryptAESGCM(keyHex, encrypted)
	if err != nil {
		t.Fatalf("DecryptAESGCM: %v", err)
	}
	if string(decrypted) != "report body" {
		t.Fatalf("DecryptAESGCM: got %q, want %q", decrypted, "report body")
	}
}

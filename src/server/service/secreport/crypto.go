// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — encryption at rest for report bodies.
// PGP is used when a project keypair exists; otherwise AES-256-GCM under
// server.security.encryption_key (the spec's canonical at-rest AES key).
// Plaintext is NEVER persisted to disk.
package secreport

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/apimgr/vidveil/src/server/service/pgp"
)

// EncryptionMethod identifies how a report body was encrypted at rest.
type EncryptionMethod string

const (
	// EncryptionMethodPGP means the body was encrypted to the project's PGP
	// public key.
	EncryptionMethodPGP EncryptionMethod = "pgp"
	// EncryptionMethodAES means the body was encrypted with AES-256-GCM
	// under server.security.encryption_key (no PGP keypair configured).
	EncryptionMethodAES EncryptionMethod = "aes-256-gcm"
)

// EncryptReportBody encrypts body with the project's PGP public key when one
// exists under configDir; otherwise falls back to AES-256-GCM using
// encryptionKeyHex (server.security.encryption_key).
func EncryptReportBody(configDir, encryptionKeyHex string, body []byte) ([]byte, EncryptionMethod, error) {
	if pub, err := pgp.LoadPublicKey(configDir); err == nil && len(pub) > 0 {
		encrypted, encErr := pgp.EncryptMessageToPublicKey(pub, body)
		if encErr == nil {
			return encrypted, EncryptionMethodPGP, nil
		}
	}
	encrypted, err := EncryptAESGCM(encryptionKeyHex, body)
	if err != nil {
		return nil, "", fmt.Errorf("encrypt report body: %w", err)
	}
	return encrypted, EncryptionMethodAES, nil
}

// EncryptAESGCM encrypts plaintext under a hex-encoded 32-byte AES-256 key.
// The nonce is prepended to the returned ciphertext.
func EncryptAESGCM(hexKey string, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(hexKey)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAESGCM decrypts ciphertext produced by EncryptAESGCM.
func DecryptAESGCM(hexKey string, ciphertext []byte) ([]byte, error) {
	gcm, err := newGCM(hexKey)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

func newGCM(hexKey string) (cipher.AEAD, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

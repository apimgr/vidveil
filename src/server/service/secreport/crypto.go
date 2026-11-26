// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — encryption at rest for report bodies.
// PGP is used when a project keypair exists; otherwise AES-256-GCM under
// server.security.encryption_key (the spec's canonical at-rest AES key).
// Plaintext is NEVER persisted to disk.
package secreport

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/apimgr/vidveil/src/server/service/pgp"
)

// EncryptionKeyCurrentFingerprint is the non-secret DB marker used to stop a
// live process from persisting ciphertext under an obsolete config key.
const EncryptionKeyCurrentFingerprint = "encryption_key_current_fingerprint"

// EncryptionKeyFingerprint returns the non-reversible key-generation marker.
func EncryptionKeyFingerprint(key string) string {
	fingerprint := sha256.Sum256([]byte(key))
	return hex.EncodeToString(fingerprint[:])
}

type encryptionKeyQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// ValidateCurrentEncryptionKey rejects at-rest writes from a process whose
// in-memory key no longer matches the database's committed key generation.
func ValidateCurrentEncryptionKey(ctx context.Context, queryer encryptionKeyQueryer, key string) error {
	var expected string
	err := queryer.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = ?`, EncryptionKeyCurrentFingerprint).Scan(&expected)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read current encryption-key marker: %w", err)
	}
	if EncryptionKeyFingerprint(key) != expected {
		return fmt.Errorf("stale encryption key: reload server configuration before writing encrypted data")
	}
	return nil
}

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
		if encErr != nil {
			return nil, "", fmt.Errorf("encrypt report body with configured PGP key: %w", encErr)
		}
		return encrypted, EncryptionMethodPGP, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, "", fmt.Errorf("load configured PGP key: %w", err)
	}
	encrypted, err := EncryptAESGCM(encryptionKeyHex, body)
	if err != nil {
		return nil, "", fmt.Errorf("encrypt report body: %w", err)
	}
	return encrypted, EncryptionMethodAES, nil
}

// DecryptReportBody decrypts only the stored report body. Contact details use
// AES-GCM independently, even when the report body is PGP-encrypted.
func DecryptReportBody(configDir string, installationSecret []byte, encryptionKeyHex string, method EncryptionMethod, ciphertext []byte) ([]byte, error) {
	switch method {
	case EncryptionMethodAES:
		return DecryptAESGCM(encryptionKeyHex, ciphertext)
	case EncryptionMethodPGP:
		private, err := pgp.LoadPrivateKey(configDir, installationSecret)
		if err != nil {
			return nil, fmt.Errorf("load PGP private key: %w", err)
		}
		return DecryptPGPReportBody(private, ciphertext)
	default:
		return nil, fmt.Errorf("unsupported report body algorithm %q", method)
	}
}

// DecryptPGPReportBody decrypts a PGP report using an already decrypted
// armored private key. It is used during atomic key rotation.
func DecryptPGPReportBody(private, ciphertext []byte) ([]byte, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(private))
	if err != nil {
		return nil, fmt.Errorf("read PGP private key: %w", err)
	}
	block, err := armor.Decode(bytes.NewReader(ciphertext))
	if err != nil {
		return nil, fmt.Errorf("decode armored report body: %w", err)
	}
	message, err := openpgp.ReadMessage(block.Body, entities, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt report body: %w", err)
	}
	body, err := io.ReadAll(message.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("read decrypted report body: %w", err)
	}
	if err := message.SignatureError; err != nil {
		return nil, fmt.Errorf("verify report body signature: %w", err)
	}
	return body, nil
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

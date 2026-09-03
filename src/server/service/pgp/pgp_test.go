package pgp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// TestGenerateKeypair verifies a keypair is produced with the expected identity,
// fingerprint, and validity window, and that both armored blocks parse.
func TestGenerateKeypair(t *testing.T) {
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if kp.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	if len(kp.PublicArmored) == 0 || len(kp.PrivateArmored) == 0 {
		t.Fatal("empty armored keys")
	}
	if !kp.ExpiresAt.After(kp.CreatedAt) {
		t.Fatalf("expiry %v not after creation %v", kp.ExpiresAt, kp.CreatedAt)
	}
	want := kp.CreatedAt.Add(DefaultValidity)
	if d := kp.ExpiresAt.Sub(want); d > time.Minute || d < -time.Minute {
		t.Fatalf("expiry %v not ~2 years after creation", kp.ExpiresAt)
	}

	el, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(kp.PublicArmored))
	if err != nil {
		t.Fatalf("parse public: %v", err)
	}
	if len(el) != 1 {
		t.Fatalf("expected 1 public entity, got %d", len(el))
	}
	ids := el[0].Identities
	found := false
	for name := range ids {
		if name == "VidVeil Security <security@example.com>" {
			found = true
		}
	}
	if !found {
		t.Fatalf("identity not found in %v", ids)
	}

	if _, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(kp.PrivateArmored)); err != nil {
		t.Fatalf("parse private: %v", err)
	}
}

// TestEncryptDecryptRoundtrip verifies the private-key at-rest encryption
// roundtrips and that a wrong secret fails to decrypt.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	secret := []byte("installation-secret-value")
	plain := []byte("-----BEGIN PGP PRIVATE KEY BLOCK-----\nfake\n-----END-----\n")

	enc, err := EncryptPrivateKey(plain, secret)
	if err != nil {
		t.Fatalf("EncryptPrivateKey: %v", err)
	}
	if bytes.Contains(enc, plain) {
		t.Fatal("ciphertext contains plaintext")
	}

	got, err := DecryptPrivateKey(enc, secret)
	if err != nil {
		t.Fatalf("DecryptPrivateKey: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("roundtrip mismatch: got %q", got)
	}

	if _, err := DecryptPrivateKey(enc, []byte("wrong-secret")); err == nil {
		t.Fatal("expected decryption failure with wrong secret")
	}
}

// TestLoadMissingKeys verifies loaders error cleanly when no keypair exists.
func TestLoadMissingKeys(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadPublicKey(dir); err == nil {
		t.Fatal("expected error loading missing public key")
	}
	if _, err := LoadPrivateKey(dir, []byte("secret")); err == nil {
		t.Fatal("expected error loading missing private key")
	}
}

// TestWriteKeypairMkdirFails verifies a MkdirAll failure is surfaced. A regular
// file occupying the config path makes SecurityDir creation impossible.
func TestWriteKeypairMkdirFails(t *testing.T) {
	base := t.TempDir()
	fileAsConfigDir := filepath.Join(base, "notadir")
	if err := os.WriteFile(fileAsConfigDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if err := WriteKeypair(fileAsConfigDir, kp, []byte("secret")); err == nil {
		t.Fatal("expected WriteKeypair to fail when config path is a file")
	}
}

// TestGenerateKeypairDefaultValidity verifies a non-positive validity falls back
// to DefaultValidity.
func TestGenerateKeypairDefaultValidity(t *testing.T) {
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", 0)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	want := kp.CreatedAt.Add(DefaultValidity)
	if d := kp.ExpiresAt.Sub(want); d > time.Minute || d < -time.Minute {
		t.Fatalf("expiry %v not ~DefaultValidity after creation", kp.ExpiresAt)
	}
}

// TestDecryptPrivateKeyShort verifies short/corrupt blobs are rejected.
func TestDecryptPrivateKeyShort(t *testing.T) {
	secret := []byte("installation-secret-value")
	if _, err := DecryptPrivateKey([]byte("short"), secret); err == nil {
		t.Fatal("expected error for short blob")
	}
	// Long enough to pass the salt+nonce length gate but not valid GCM.
	if _, err := DecryptPrivateKey(make([]byte, saltLen+12+4), secret); err == nil {
		t.Fatal("expected error for corrupt ciphertext")
	}
}

// TestWriteAndLoadKeypair verifies files are written with the right modes and
// that the public/private keys load back correctly.
func TestWriteAndLoadKeypair(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("installation-secret-value")

	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if err := WriteKeypair(dir, kp, secret); err != nil {
		t.Fatalf("WriteKeypair: %v", err)
	}

	pubPath := filepath.Join(SecurityDir(dir), PublicKeyFile)
	privPath := filepath.Join(SecurityDir(dir), PrivateKeyFile)

	pi, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("stat private: %v", err)
	}
	if pi.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o, want 600", pi.Mode().Perm())
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("stat public: %v", err)
	}

	pub, err := LoadPublicKey(dir)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if !bytes.Equal(pub, kp.PublicArmored) {
		t.Fatal("loaded public key mismatch")
	}

	priv, err := LoadPrivateKey(dir, secret)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	if !bytes.Equal(priv, kp.PrivateArmored) {
		t.Fatal("loaded private key mismatch")
	}
}

// TestDeleteKeypair verifies both key files (and the keyserver state) are removed
// and that a second delete is a no-op error-free (AI.md PART 12 "Delete").
func TestDeleteKeypair(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("installation-secret-value")
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if err := WriteKeypair(dir, kp, secret); err != nil {
		t.Fatalf("WriteKeypair: %v", err)
	}
	statePath := filepath.Join(SecurityDir(dir), keyserverStateFile)
	if err := os.WriteFile(statePath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	if err := DeleteKeypair(dir); err != nil {
		t.Fatalf("DeleteKeypair: %v", err)
	}
	for _, name := range []string{PublicKeyFile, PrivateKeyFile, keyserverStateFile} {
		if _, err := os.Stat(filepath.Join(SecurityDir(dir), name)); !os.IsNotExist(err) {
			t.Fatalf("%s still present after delete: %v", name, err)
		}
	}

	// Idempotent: deleting again when files are gone is not an error.
	if err := DeleteKeypair(dir); err != nil {
		t.Fatalf("second DeleteKeypair should be a no-op: %v", err)
	}
}

// TestDeleteKeypairRemoveError verifies a non-NotExist removal failure is
// surfaced. A non-empty directory occupying a key file path makes os.Remove fail
// with something other than "not exist" even for the root test user.
func TestDeleteKeypairRemoveError(t *testing.T) {
	dir := t.TempDir()
	secDir := SecurityDir(dir)
	if err := os.MkdirAll(filepath.Join(secDir, PublicKeyFile, "child"), 0o700); err != nil {
		t.Fatalf("setup non-empty dir at key path: %v", err)
	}
	if err := DeleteKeypair(dir); err == nil {
		t.Fatal("expected DeleteKeypair to fail removing a non-empty directory")
	}
}

// TestParsePrivateKey verifies that a generated private key round-trips through
// ParsePrivateKey with matching fingerprint, identity, and validity window, and
// that the derived public key re-parses. Powers the import flow (AI.md PART 12).
func TestParsePrivateKey(t *testing.T) {
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	got, name, email, err := ParsePrivateKey(kp.PrivateArmored)
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}
	if got.Fingerprint != kp.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", got.Fingerprint, kp.Fingerprint)
	}
	if name != "VidVeil Security" {
		t.Fatalf("name = %q, want %q", name, "VidVeil Security")
	}
	if email != "security@example.com" {
		t.Fatalf("email = %q, want %q", email, "security@example.com")
	}
	if d := got.ExpiresAt.Sub(kp.CreatedAt.Add(DefaultValidity)); d > time.Minute || d < -time.Minute {
		t.Fatalf("expiry %v not ~2 years after creation", got.ExpiresAt)
	}
	if !bytes.Equal(got.PrivateArmored, kp.PrivateArmored) {
		t.Fatal("private armored mismatch")
	}
	if _, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(got.PublicArmored)); err != nil {
		t.Fatalf("derived public key does not parse: %v", err)
	}
}

// TestParsePrivateKeyRejectsPublic verifies a public-only armored block is
// rejected (no private key material to import).
func TestParsePrivateKeyRejectsPublic(t *testing.T) {
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if _, _, _, err := ParsePrivateKey(kp.PublicArmored); err == nil {
		t.Fatal("expected error parsing a public-only block as private")
	}
}

// TestParsePrivateKeyGarbage verifies non-armored input is rejected.
func TestParsePrivateKeyGarbage(t *testing.T) {
	if _, _, _, err := ParsePrivateKey([]byte("not a pgp key")); err == nil {
		t.Fatal("expected error parsing garbage input")
	}
}

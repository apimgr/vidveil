package pgp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// TestCrossSignPublicKey verifies the new public key is certified by the old
// key: after cross-signing, an identity signature issued by the old key's
// primary key id is present on the re-armored new public key.
func TestCrossSignPublicKey(t *testing.T) {
	oldKP, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("generate old: %v", err)
	}
	newKP, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("generate new: %v", err)
	}

	signed, err := CrossSignPublicKey(newKP.PublicArmored, oldKP.PrivateArmored)
	if err != nil {
		t.Fatalf("CrossSignPublicKey: %v", err)
	}

	oldRing, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(oldKP.PublicArmored))
	if err != nil {
		t.Fatalf("parse old public: %v", err)
	}
	oldKeyID := oldRing[0].PrimaryKey.KeyId

	ring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(signed))
	if err != nil {
		t.Fatalf("parse signed public: %v", err)
	}
	if len(ring) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(ring))
	}
	found := false
	for _, id := range ring[0].Identities {
		for _, sig := range id.Signatures {
			if sig.IssuerKeyId != nil && *sig.IssuerKeyId == oldKeyID {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no cross-certification from old key found on new public key")
	}
}

// TestCrossSignPublicKeyBadInput verifies malformed inputs are rejected.
func TestCrossSignPublicKeyBadInput(t *testing.T) {
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := CrossSignPublicKey([]byte("not armored"), kp.PrivateArmored); err == nil {
		t.Fatal("expected error for bad new public key")
	}
	if _, err := CrossSignPublicKey(kp.PublicArmored, []byte("not armored")); err == nil {
		t.Fatal("expected error for bad old private key")
	}
}

// TestArchiveCurrentKeys verifies the current keypair files move into the
// rotated/ archive with a grace marker recording the valid-until timestamp.
func TestArchiveCurrentKeys(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("installation-secret-value")
	kp, err := GenerateKeypair("VidVeil Security", "security@example.com", DefaultValidity)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := WriteKeypair(dir, kp, secret); err != nil {
		t.Fatalf("WriteKeypair: %v", err)
	}

	rotatedAt := time.Now()
	archiveDir, err := ArchiveCurrentKeys(dir, kp.Fingerprint, rotatedAt, RotationGracePeriod)
	if err != nil {
		t.Fatalf("ArchiveCurrentKeys: %v", err)
	}

	// Live key files are gone.
	if _, err := os.Stat(filepath.Join(SecurityDir(dir), PrivateKeyFile)); !os.IsNotExist(err) {
		t.Fatal("expected live private key to be moved out")
	}
	// Archived copies exist.
	if _, err := os.Stat(filepath.Join(archiveDir, PrivateKeyFile)); err != nil {
		t.Fatalf("archived private key missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, PublicKeyFile)); err != nil {
		t.Fatalf("archived public key missing: %v", err)
	}
	marker, err := os.ReadFile(filepath.Join(archiveDir, graceMarkerFile))
	if err != nil {
		t.Fatalf("read grace marker: %v", err)
	}
	want := rotatedAt.Add(RotationGracePeriod).UTC().Format(time.RFC3339)
	if string(bytes.TrimSpace(marker)) != want {
		t.Fatalf("grace marker = %q, want %q", bytes.TrimSpace(marker), want)
	}
}

// TestArchiveCurrentKeysNoKey verifies archiving errors when no private key
// exists to archive.
func TestArchiveCurrentKeysNoKey(t *testing.T) {
	dir := t.TempDir()
	if _, err := ArchiveCurrentKeys(dir, "ABCD", time.Now(), RotationGracePeriod); err == nil {
		t.Fatal("expected error archiving with no existing private key")
	}
}

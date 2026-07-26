package pgp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// RotationGracePeriod is how long the previous keypair remains available for
// decrypting in-flight security reports after a rotation (AI.md PART 12
// "Rotate": "Old key stays valid for 30 days for in-flight reports").
const RotationGracePeriod = 30 * 24 * time.Hour

// RotatedDir is the subdirectory under {config_dir}/security/ that holds
// archived keypairs retained through the rotation grace window.
const RotatedDir = "rotated"

// graceMarkerFile records the UTC RFC 3339 timestamp until which an archived
// keypair remains valid for in-flight report decryption.
const graceMarkerFile = "valid_until"

// CrossSignPublicKey adds a certification from the old private key over every
// identity of the new public key, then returns the re-armored new public key.
// This lets anyone who trusts the old key trust the new one (AI.md PART 12
// "Rotate": "signs the new pubkey with the old key"). The old private key must
// be an unencrypted (passphrase-less) armored private key.
func CrossSignPublicKey(newPubArmored, oldPrivArmored []byte) ([]byte, error) {
	oldRing, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(oldPrivArmored))
	if err != nil {
		return nil, fmt.Errorf("read old private key: %w", err)
	}
	if len(oldRing) == 0 {
		return nil, fmt.Errorf("old private key ring is empty")
	}
	newRing, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(newPubArmored))
	if err != nil {
		return nil, fmt.Errorf("read new public key: %w", err)
	}
	if len(newRing) == 0 {
		return nil, fmt.Errorf("new public key ring is empty")
	}
	oldEntity := oldRing[0]
	newEntity := newRing[0]

	cfg := &packet.Config{Time: time.Now}
	signed := false
	for id := range newEntity.Identities {
		if err := newEntity.SignIdentity(id, oldEntity, cfg); err != nil {
			return nil, fmt.Errorf("cross-sign identity %q: %w", id, err)
		}
		signed = true
	}
	if !signed {
		return nil, fmt.Errorf("new public key has no identities to sign")
	}

	var buf bytes.Buffer
	aw, err := armor.Encode(&buf, "PGP PUBLIC KEY BLOCK", nil)
	if err != nil {
		return nil, fmt.Errorf("armor new public key: %w", err)
	}
	if err := newEntity.Serialize(aw); err != nil {
		_ = aw.Close()
		return nil, fmt.Errorf("serialize new public key: %w", err)
	}
	if err := aw.Close(); err != nil {
		return nil, fmt.Errorf("close armor: %w", err)
	}
	return buf.Bytes(), nil
}

// ArchiveCurrentKeys moves the current public and encrypted private key files
// into {config_dir}/security/rotated/{label}/ and records the grace-window
// expiry, so reports encrypted to the previous key stay decryptable for
// RotationGracePeriod after a rotation (AI.md PART 12 "Rotate"). It returns the
// archive directory path.
func ArchiveCurrentKeys(configDir, fingerprint string, rotatedAt time.Time, grace time.Duration) (string, error) {
	dir := SecurityDir(configDir)
	pubSrc := filepath.Join(dir, PublicKeyFile)
	privSrc := filepath.Join(dir, PrivateKeyFile)
	if _, err := os.Stat(privSrc); err != nil {
		return "", fmt.Errorf("no existing private key to archive: %w", err)
	}

	label := rotatedAt.UTC().Format("20060102T150405Z")
	if fingerprint != "" {
		short := fingerprint
		if len(short) > 16 {
			short = short[:16]
		}
		label = label + "-" + short
	}
	archiveDir := filepath.Join(dir, RotatedDir, label)
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}
	if err := os.Rename(privSrc, filepath.Join(archiveDir, PrivateKeyFile)); err != nil {
		return "", fmt.Errorf("archive private key: %w", err)
	}
	if _, err := os.Stat(pubSrc); err == nil {
		if err := os.Rename(pubSrc, filepath.Join(archiveDir, PublicKeyFile)); err != nil {
			return "", fmt.Errorf("archive public key: %w", err)
		}
	}
	validUntil := rotatedAt.Add(grace).UTC().Format(time.RFC3339)
	marker := filepath.Join(archiveDir, graceMarkerFile)
	if err := os.WriteFile(marker, []byte(validUntil+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write grace marker: %w", err)
	}
	return archiveDir, nil
}

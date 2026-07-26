// Package pgp implements the project-level GPG keypair used by the coordinated
// security-disclosure feature (AI.md PART 12 "GPG Keypair Management"). It
// generates an Ed25519 (signing) + Curve25519 (encryption) keypair, encrypts
// the private key at rest with a key derived from the installation secret, and
// persists the armored keys under {config_dir}/security/.
package pgp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"golang.org/x/crypto/argon2"
)

const (
	// PublicKeyFile is the armored public key, served at /.well-known/pgp-key.asc.
	PublicKeyFile = "pgp.pub.asc"
	// PrivateKeyFile is the AES-256-GCM encrypted armored private key.
	PrivateKeyFile = "pgp.priv.asc.enc"
	// DefaultValidity is the keypair lifetime (2 years) per AI.md PART 12.
	DefaultValidity = 2 * 365 * 24 * time.Hour

	saltLen = 16
)

// Keypair is a freshly generated OpenPGP keypair and its metadata.
type Keypair struct {
	// Fingerprint is the uppercase hex primary-key fingerprint.
	Fingerprint string
	// PublicArmored is the ASCII-armored public key.
	PublicArmored []byte
	// PrivateArmored is the ASCII-armored private key (unencrypted, in memory only).
	PrivateArmored []byte
	// CreatedAt is when the keypair was generated (UTC).
	CreatedAt time.Time
	// ExpiresAt is when the keypair expires (UTC).
	ExpiresAt time.Time
}

// SecurityDir returns the directory holding the keypair files for a config dir.
func SecurityDir(configDir string) string {
	return filepath.Join(configDir, "security")
}

// GenerateKeypair creates an Ed25519 + Curve25519 keypair with the given
// identity (name/email) and validity window. The identity per AI.md PART 12 is
// "{app_name} Security <{security_contact}>", so name is "{app_name} Security"
// and email is the security contact address.
func GenerateKeypair(name, email string, validity time.Duration) (*Keypair, error) {
	if validity <= 0 {
		validity = DefaultValidity
	}
	created := time.Now().UTC()
	cfg := &packet.Config{
		Algorithm:              packet.PubKeyAlgoEdDSA,
		DefaultHash:            0,
		Time:                   func() time.Time { return created },
		KeyLifetimeSecs:        uint32(validity.Seconds()),
		SigLifetimeSecs:        uint32(validity.Seconds()),
		DefaultCompressionAlgo: packet.CompressionZLIB,
	}
	entity, err := openpgp.NewEntity(name, "", email, cfg)
	if err != nil {
		return nil, fmt.Errorf("generate openpgp entity: %w", err)
	}

	pub, err := armorEntity(entity, false, cfg)
	if err != nil {
		return nil, fmt.Errorf("armor public key: %w", err)
	}
	priv, err := armorEntity(entity, true, cfg)
	if err != nil {
		return nil, fmt.Errorf("armor private key: %w", err)
	}

	return &Keypair{
		Fingerprint:    strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint)),
		PublicArmored:  pub,
		PrivateArmored: priv,
		CreatedAt:      created,
		ExpiresAt:      created.Add(validity),
	}, nil
}

// armorEntity serializes an entity (public or private) to an ASCII-armored block.
func armorEntity(entity *openpgp.Entity, private bool, cfg *packet.Config) ([]byte, error) {
	blockType := "PGP PUBLIC KEY BLOCK"
	if private {
		blockType = "PGP PRIVATE KEY BLOCK"
	}
	var buf bytes.Buffer
	aw, err := armor.Encode(&buf, blockType, nil)
	if err != nil {
		return nil, err
	}
	if private {
		if err := entity.SerializePrivate(aw, cfg); err != nil {
			_ = aw.Close()
			return nil, err
		}
	} else {
		if err := entity.Serialize(aw); err != nil {
			_ = aw.Close()
			return nil, err
		}
	}
	if err := aw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// deriveKey derives a 32-byte AES key from the installation secret and salt
// using Argon2id, matching the KDF parameters used for backup encryption.
func deriveKey(secret, salt []byte) []byte {
	return argon2.IDKey(secret, salt, 1, 64*1024, 4, 32)
}

// EncryptPrivateKey encrypts the armored private key with a key derived from the
// installation secret. Layout: salt(16) || nonce(12) || ciphertext+tag.
func EncryptPrivateKey(armoredPriv, secret []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("read salt: %w", err)
	}
	key := deriveKey(secret, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	out := make([]byte, 0, saltLen+len(nonce)+len(armoredPriv)+gcm.Overhead())
	out = append(out, salt...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, armoredPriv, nil)
	return out, nil
}

// DecryptPrivateKey reverses EncryptPrivateKey.
func DecryptPrivateKey(blob, secret []byte) ([]byte, error) {
	if len(blob) < saltLen+12 {
		return nil, fmt.Errorf("encrypted private key too short")
	}
	salt := blob[:saltLen]
	rest := blob[saltLen:]
	key := deriveKey(secret, salt)
	cb, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(cb)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(rest) < ns {
		return nil, fmt.Errorf("encrypted private key too short")
	}
	nonce := rest[:ns]
	ct := rest[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}
	return plain, nil
}

// WriteKeypair writes the public key (0644) and the encrypted private key (0600)
// to {config_dir}/security/, creating the directory if needed. The private key
// is encrypted with a key derived from the installation secret before writing.
func WriteKeypair(configDir string, kp *Keypair, secret []byte) error {
	dir := SecurityDir(configDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create security dir: %w", err)
	}
	pubPath := filepath.Join(dir, PublicKeyFile)
	if err := os.WriteFile(pubPath, kp.PublicArmored, 0o644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}
	enc, err := EncryptPrivateKey(kp.PrivateArmored, secret)
	if err != nil {
		return fmt.Errorf("encrypt private key: %w", err)
	}
	privPath := filepath.Join(dir, PrivateKeyFile)
	if err := os.WriteFile(privPath, enc, 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}
	return nil
}

// LoadPublicKey reads the armored public key from {config_dir}/security/.
func LoadPublicKey(configDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(SecurityDir(configDir), PublicKeyFile))
}

// LoadPrivateKey reads and decrypts the private key from {config_dir}/security/.
func LoadPrivateKey(configDir string, secret []byte) ([]byte, error) {
	blob, err := os.ReadFile(filepath.Join(SecurityDir(configDir), PrivateKeyFile))
	if err != nil {
		return nil, err
	}
	return DecryptPrivateKey(blob, secret)
}

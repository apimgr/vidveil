// SPDX-License-Identifier: MIT
// Coverage tests for the pgp private-key import flow (AI.md PART 12).
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// setupPGPEnv creates a config+data dir pair with the given security contact
// email and branding title, and returns the two directory paths.
func setupPGPEnv(t *testing.T, email, title string) (cfgDir, dataDir string) {
	t.Helper()
	base := t.TempDir()
	cfgDir = filepath.Join(base, "config")
	dataDir = filepath.Join(base, "data")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	cfg.Server.Contact.Security.Email = email
	cfg.Server.Branding.Title = title
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}
	return cfgDir, dataDir
}

// genPrivateKeyFile generates a keypair in a throwaway env and writes its
// decrypted armored private key to a file, returning the path and fingerprint.
func genPrivateKeyFile(t *testing.T, email, title string) (path, fingerprint string) {
	t.Helper()
	cfgDir, dataDir := setupPGPEnv(t, email, title)
	kp, _, _, _, err := pgpGenerateCore(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}
	path = filepath.Join(t.TempDir(), "priv.asc")
	if err := os.WriteFile(path, kp.PrivateArmored, 0o600); err != nil {
		t.Fatalf("write private key file: %v", err)
	}
	return path, kp.Fingerprint
}

// TestPgpImportCore_Success imports a matching-identity private key into a fresh
// environment and verifies the keypair, config flip, and public-key path.
func TestPgpImportCore_Success(t *testing.T) {
	privFile, fingerprint := genPrivateKeyFile(t, "security@example.com", "VidVeil")

	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	res, err := pgpImportCore(cfgDir, dataDir, privFile, nil)
	if err != nil {
		t.Fatalf("pgpImportCore: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result on matching identity")
	}
	if res.fingerprint != fingerprint {
		t.Fatalf("fingerprint = %q, want %q", res.fingerprint, fingerprint)
	}
	if res.identityName != "VidVeil Security" || res.identityEmail != "security@example.com" {
		t.Fatalf("identity = %q <%s>", res.identityName, res.identityEmail)
	}
	if _, err := os.Stat(res.pubKeyPath); err != nil {
		t.Fatalf("public key not written at %s: %v", res.pubKeyPath, err)
	}

	cfg, _, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !cfg.Web.Security.PublishPGPKey {
		t.Fatal("expected PublishPGPKey enabled after import")
	}
}

// TestPgpImportCore_MismatchDeclined verifies a declined identity mismatch aborts
// with a nil result and nil error (the operator chose not to override).
func TestPgpImportCore_MismatchDeclined(t *testing.T) {
	privFile, _ := genPrivateKeyFile(t, "security@example.com", "VidVeil")

	cfgDir, dataDir := setupPGPEnv(t, "other@example.com", "VidVeil")
	res, err := pgpImportCore(cfgDir, dataDir, privFile, func(expected, got string) bool { return false })
	if err != nil {
		t.Fatalf("pgpImportCore: %v", err)
	}
	if res != nil {
		t.Fatal("expected nil result when mismatch is declined")
	}
}

// TestPgpImportCore_MismatchOverridden verifies an overridden identity mismatch
// proceeds with the import.
func TestPgpImportCore_MismatchOverridden(t *testing.T) {
	privFile, fingerprint := genPrivateKeyFile(t, "security@example.com", "VidVeil")

	cfgDir, dataDir := setupPGPEnv(t, "other@example.com", "VidVeil")
	called := false
	res, err := pgpImportCore(cfgDir, dataDir, privFile, func(expected, got string) bool {
		called = true
		return true
	})
	if err != nil {
		t.Fatalf("pgpImportCore: %v", err)
	}
	if !called {
		t.Fatal("expected confirmMismatch to be called on identity mismatch")
	}
	if res == nil || res.fingerprint != fingerprint {
		t.Fatalf("expected successful import after override, got %+v", res)
	}
}

// TestPgpImportCore_BadFile verifies a missing source file is surfaced as an error.
func TestPgpImportCore_BadFile(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	if _, err := pgpImportCore(cfgDir, dataDir, filepath.Join(t.TempDir(), "nope.asc"), nil); err == nil {
		t.Fatal("expected error for missing source file")
	}
}

// TestPgpImportCore_GarbageFile verifies a non-armored file fails to parse.
func TestPgpImportCore_GarbageFile(t *testing.T) {
	cfgDir, dataDir := setupPGPEnv(t, "security@example.com", "VidVeil")
	bad := filepath.Join(t.TempDir(), "bad.asc")
	if err := os.WriteFile(bad, []byte("not a pgp key"), 0o600); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if _, err := pgpImportCore(cfgDir, dataDir, bad, nil); err == nil {
		t.Fatal("expected error parsing garbage private key")
	}
}

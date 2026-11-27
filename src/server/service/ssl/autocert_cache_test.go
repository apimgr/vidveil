// SPDX-License-Identifier: MIT
package ssl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/acme/autocert"
)

func TestManagedACMECacheStoresCertificateAtConfiguredPaths(t *testing.T) {
	configDir := t.TempDir()
	manager := newEnabledSSLManager(t)
	if err := manager.generateSelfSigned(); err != nil {
		t.Fatalf("generate self-signed fixture: %v", err)
	}
	privateKey, err := os.ReadFile(filepath.Join(manager.certPath, "key.pem"))
	if err != nil {
		t.Fatalf("read private key fixture: %v", err)
	}
	certificate, err := os.ReadFile(filepath.Join(manager.certPath, "cert.pem"))
	if err != nil {
		t.Fatalf("read certificate fixture: %v", err)
	}

	cache := newManagedACMECache(configDir, "test.example")
	cacheValue := append(append([]byte(nil), privateKey...), certificate...)
	if err := cache.Put(context.Background(), "test.example", cacheValue); err != nil {
		t.Fatalf("store managed certificate: %v", err)
	}
	got, err := cache.Get(context.Background(), "test.example")
	if err != nil {
		t.Fatalf("read managed certificate: %v", err)
	}
	if !bytes.Equal(got, cacheValue) {
		t.Fatal("managed certificate cache did not round-trip key and chain")
	}
	upperCase, err := cache.Get(context.Background(), "TEST.EXAMPLE.")
	if err != nil || !bytes.Equal(upperCase, cacheValue) {
		t.Errorf("normalized FQDN cache lookup returned error %v or mismatched data", err)
	}
	for _, path := range []string{cache.cert, cache.key} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat managed certificate file %q: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("managed file %q permissions = %o, want 600", path, info.Mode().Perm())
		}
	}

	if err := cache.Delete(context.Background(), "test.example"); err != nil {
		t.Fatalf("delete managed certificate: %v", err)
	}
	if _, err := cache.Get(context.Background(), "test.example"); !errors.Is(err, autocert.ErrCacheMiss) {
		t.Errorf("read after deleting managed certificate error = %v, want cache miss", err)
	}
}

func TestManagedACMECacheKeepsAutocertStateSeparate(t *testing.T) {
	cache := newManagedACMECache(t.TempDir(), "test.example")
	key := "acme_account+key"
	want := []byte("opaque account state")
	if err := cache.Put(context.Background(), key, want); err != nil {
		t.Fatalf("store autocert account state: %v", err)
	}
	got, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("read autocert account state: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("autocert account state = %q, want %q", got, want)
	}
}

func TestManagedACMECacheRejectsMismatchedPairWithoutChangingStoredPair(t *testing.T) {
	first := newEnabledSSLManager(t)
	if err := first.generateSelfSigned(); err != nil {
		t.Fatal(err)
	}
	firstKey, err := os.ReadFile(filepath.Join(first.certPath, "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	firstCert, err := os.ReadFile(filepath.Join(first.certPath, "cert.pem"))
	if err != nil {
		t.Fatal(err)
	}
	second := newEnabledSSLManager(t)
	if err := second.generateSelfSigned(); err != nil {
		t.Fatal(err)
	}
	secondCert, err := os.ReadFile(filepath.Join(second.certPath, "cert.pem"))
	if err != nil {
		t.Fatal(err)
	}

	cache := newManagedACMECache(t.TempDir(), "test.example")
	want := append(append([]byte(nil), firstKey...), firstCert...)
	if err := cache.Put(context.Background(), "test.example", want); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(context.Background(), "test.example", append(append([]byte(nil), firstKey...), secondCert...)); err == nil {
		t.Fatal("cache accepted a mismatched certificate and private key")
	}
	got, err := cache.Get(context.Background(), "test.example")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("rejected cache update changed the active certificate pair")
	}
}

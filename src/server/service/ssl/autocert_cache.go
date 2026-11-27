// SPDX-License-Identifier: MIT
package ssl

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/acme/autocert"
)

// managedACMECache stores the configured domain certificate in PART 15's
// public certificate and private-key paths while keeping autocert account and
// challenge state in a private auxiliary cache.
type managedACMECache struct {
	domain string
	cert   string
	key    string
	state  autocert.DirCache
	mu     sync.Mutex
}

func newManagedACMECache(configDir, domain string) *managedACMECache {
	domain = normalizeACMECacheKey(domain)
	base := filepath.Join(configDir, "ssl", "letsencrypt", domain)
	return &managedACMECache{
		domain: domain,
		cert:   filepath.Join(base, "fullchain.pem"),
		key:    filepath.Join(base, "privkey.pem"),
		state:  autocert.DirCache(filepath.Join(configDir, "ssl", "acme-state")),
	}
}

func normalizeACMECacheKey(key string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(key)), ".")
}

func (c *managedACMECache) Get(ctx context.Context, key string) ([]byte, error) {
	if normalizeACMECacheKey(key) != c.domain {
		return c.state.Get(ctx, key)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	privateKey, err := os.ReadFile(c.key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, autocert.ErrCacheMiss
		}
		return nil, fmt.Errorf("read managed certificate key: %w", err)
	}
	certificate, err := os.ReadFile(c.cert)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, autocert.ErrCacheMiss
		}
		return nil, fmt.Errorf("read managed certificate chain: %w", err)
	}
	return append(append([]byte(nil), privateKey...), certificate...), nil
}

func (c *managedACMECache) Put(ctx context.Context, key string, data []byte) (retErr error) {
	if normalizeACMECacheKey(key) != c.domain {
		return c.state.Put(ctx, key, data)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	block, remainder := pem.Decode(data)
	if block == nil || !strings.Contains(block.Type, "PRIVATE") || len(bytes.TrimSpace(remainder)) == 0 {
		return fmt.Errorf("autocert returned an invalid certificate cache value")
	}
	keyData := pem.EncodeToMemory(block)
	if _, err := tls.X509KeyPair(remainder, keyData); err != nil {
		return fmt.Errorf("autocert returned a mismatched certificate and private key: %w", err)
	}
	keyTemp, err := stagePrivateFile(c.key, keyData)
	if err != nil {
		log.Printf("ERROR: failed to stage managed ACME private key: %v", err)
		return fmt.Errorf("stage managed certificate key: %w", err)
	}
	defer func() {
		if err := os.Remove(keyTemp); err != nil && !errors.Is(err, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove staged managed certificate key: %w", err))
		}
	}()
	certTemp, err := stagePrivateFile(c.cert, remainder)
	if err != nil {
		log.Printf("ERROR: failed to stage managed ACME certificate chain: %v", err)
		return fmt.Errorf("stage managed certificate chain: %w", err)
	}
	defer func() {
		if err := os.Remove(certTemp); err != nil && !errors.Is(err, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove staged managed certificate chain: %w", err))
		}
	}()
	previousKey, keyReadErr := os.ReadFile(c.key)
	hadPreviousKey := keyReadErr == nil
	if keyReadErr != nil && !errors.Is(keyReadErr, os.ErrNotExist) {
		return fmt.Errorf("preserve managed certificate key: %w", keyReadErr)
	}
	if err := os.Rename(keyTemp, c.key); err != nil {
		return fmt.Errorf("replace managed certificate key: %w", err)
	}
	if err := os.Rename(certTemp, c.cert); err != nil {
		rollbackErr := error(nil)
		if hadPreviousKey {
			rollbackErr = writePrivateFile(c.key, previousKey)
		} else {
			rollbackErr = os.Remove(c.key)
			if errors.Is(rollbackErr, os.ErrNotExist) {
				rollbackErr = nil
			}
		}
		log.Printf("ERROR: failed to persist managed ACME certificate chain: %v", err)
		return errors.Join(fmt.Errorf("replace managed certificate chain: %w", err), rollbackErr)
	}
	return nil
}

func (c *managedACMECache) Delete(ctx context.Context, key string) error {
	if normalizeACMECacheKey(key) != c.domain {
		return c.state.Delete(ctx, key)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, path := range []string{c.cert, c.key} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove managed certificate file: %w", err)
		}
	}
	return nil
}

func writePrivateFile(path string, data []byte) (retErr error) {
	tmp, err := stagePrivateFile(path, data)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove temporary private file: %w", err))
		}
	}()
	return os.Rename(tmp, path)
}

func stagePrivateFile(path string, data []byte) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, ".certificate-*")
	if err != nil {
		return "", err
	}
	tmp := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return tmp, nil
}

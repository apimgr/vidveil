package pgp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KeyserverStateFile records, per keyserver, the last successful publish so a
// restore does not double-submit (AI.md PART 21 "Backup Contents").
const KeyserverStateFile = "keyservers.state"

// keyserverMaxAttempts bounds the per-keyserver submission attempts. AI.md
// PART 12 "Publish to keyservers": failures are retried with exponential
// backoff.
const keyserverMaxAttempts = 3

// keyserverHTTPClient is the HTTP client used for keyserver submissions. It has
// a bounded timeout so a hung keyserver cannot block the maintenance command.
var keyserverHTTPClient = &http.Client{Timeout: 30 * time.Second}

// keyserverRetryBackoff is the base delay for the exponential backoff between
// retry attempts. It is a package var so tests can shorten it.
var keyserverRetryBackoff = 2 * time.Second

// PublishToKeyservers submits the armored public key to each configured
// keyserver (AI.md PART 12 "Publish to keyservers"). It returns the states for
// servers that accepted the key and a combined error describing any failures;
// partial success is possible, so callers should persist the returned states
// even when err is non-nil.
func PublishToKeyservers(ctx context.Context, pubArmored []byte, keyservers []string) ([]KeyserverState, error) {
	var (
		states []KeyserverState
		errs   []string
	)
	for _, ks := range keyservers {
		ks = strings.TrimSpace(ks)
		if ks == "" {
			continue
		}
		if err := submitWithRetry(ctx, ks, pubArmored); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ks, err))
			continue
		}
		states = append(states, KeyserverState{URL: ks, PublishedAt: time.Now().UTC()})
	}
	if len(errs) > 0 {
		return states, fmt.Errorf("keyserver publish failures: %s", strings.Join(errs, "; "))
	}
	return states, nil
}

// submitWithRetry submits the public key to a single keyserver, retrying failed
// attempts with exponential backoff up to keyserverMaxAttempts. It stops early
// when the context is cancelled.
func submitWithRetry(ctx context.Context, keyserver string, pubArmored []byte) error {
	var lastErr error
	for attempt := 0; attempt < keyserverMaxAttempts; attempt++ {
		if attempt > 0 {
			delay := keyserverRetryBackoff * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return fmt.Errorf("cancelled after %d attempt(s): %w", attempt, ctx.Err())
			case <-time.After(delay):
			}
		}
		if lastErr = submitToKeyserver(ctx, keyserver, pubArmored); lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("gave up after %d attempts: %w", keyserverMaxAttempts, lastErr)
}

// submitToKeyserver POSTs the public key to a single keyserver. keys.openpgp.org
// (and Hagrid-based servers) accept a JSON body at /vks/v1/upload; classic HKP
// servers accept a urlencoded keytext form at /pks/add.
func submitToKeyserver(ctx context.Context, keyserver string, pubArmored []byte) error {
	base, err := url.Parse(strings.TrimRight(keyserver, "/"))
	if err != nil {
		return fmt.Errorf("parse keyserver url: %w", err)
	}

	var req *http.Request
	if strings.Contains(base.Host, "keys.openpgp.org") {
		body, mErr := json.Marshal(map[string]string{"keytext": string(pubArmored)})
		if mErr != nil {
			return fmt.Errorf("encode upload body: %w", mErr)
		}
		endpoint := base.String() + "/vks/v1/upload"
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		form := url.Values{"keytext": {string(pubArmored)}}
		endpoint := base.String() + "/pks/add"
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := keyserverHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("keyserver returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// WriteKeyserverState persists per-keyserver publish state to
// {config_dir}/security/keyservers.state (0600) so a later restore does not
// double-submit (AI.md PART 21).
func WriteKeyserverState(configDir string, states []KeyserverState) error {
	dir := SecurityDir(configDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create security dir: %w", err)
	}
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("encode keyserver state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, KeyserverStateFile), data, 0o600); err != nil {
		return fmt.Errorf("write keyserver state: %w", err)
	}
	return nil
}

// LoadKeyserverState reads the persisted per-keyserver publish state. A missing
// file returns an empty slice with no error.
func LoadKeyserverState(configDir string) ([]KeyserverState, error) {
	data, err := os.ReadFile(filepath.Join(SecurityDir(configDir), KeyserverStateFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read keyserver state: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var states []KeyserverState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("decode keyserver state: %w", err)
	}
	return states, nil
}

// UpdateKeyserversPublished writes the given keyserver states into the
// keyservers_published column of the live pgp_keypair row.
func UpdateKeyserversPublished(db *sql.DB, states []KeyserverState) error {
	data, err := json.Marshal(states)
	if err != nil {
		return fmt.Errorf("encode keyservers_published: %w", err)
	}
	if _, err := db.Exec(
		`UPDATE pgp_keypair SET keyservers_published = ?
		 WHERE id = (SELECT id FROM pgp_keypair ORDER BY id DESC LIMIT 1)`,
		string(data),
	); err != nil {
		return fmt.Errorf("update keyservers_published: %w", err)
	}
	return nil
}

package pgp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/server/service/database"
)

// keyserverStateFile records, per keyserver, the last successful publish so a
// restore does not double-submit (AI.md PART 21 "Backup Contents").
const keyserverStateFile = "keyservers.state"

// keyserverMaxAttempts bounds durable per-keyserver attempts per fingerprint.
const keyserverMaxAttempts = 8

// keyserverHTTPClient is the HTTP client used for keyserver submissions. It has
// a bounded timeout so a hung keyserver cannot block the maintenance command.
var keyserverHTTPClient = &http.Client{Timeout: 30 * time.Second}

// keyserverRetryBackoff is the base delay between durable retry attempts.
const keyserverRetryBackoff = time.Minute

// PublishToKeyservers submits the armored public key to each configured
// keyserver (AI.md PART 12 "Publish to keyservers"). It returns the states for
// servers that accepted the key and a combined error describing any failures;
// partial success is possible, so callers should persist the returned states
// even when err is non-nil.
func PublishToKeyservers(ctx context.Context, pubArmored []byte, keyservers []string) ([]KeyserverState, error) {
	_, published, err := PublishToKeyserversWithState(ctx, pubArmored, "", keyservers, nil)
	return published, err
}

// PublishToKeyserversWithState makes one attempt for each due keyserver and
// returns the complete fingerprint-scoped retry state for durable storage.
func PublishToKeyserversWithState(ctx context.Context, pubArmored []byte, fingerprint string, keyservers []string, previous []KeyserverState) ([]KeyserverState, []KeyserverState, error) {
	var (
		states    = append([]KeyserverState(nil), previous...)
		published []KeyserverState
		errs      []string
	)
	for _, ks := range keyservers {
		ks = strings.TrimSpace(ks)
		if ks == "" {
			continue
		}
		index := findKeyserverState(states, fingerprint, ks)
		state := KeyserverState{URL: ks, Fingerprint: fingerprint}
		if index >= 0 {
			state = states[index]
			if !state.PublishedAt.IsZero() || state.Permanent || state.Attempts >= keyserverMaxAttempts || (state.NextAttempt != nil && time.Now().Before(*state.NextAttempt)) {
				continue
			}
		}
		if err := submitToKeyserver(ctx, ks, pubArmored); err != nil {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return states, published, fmt.Errorf("keyserver submission cancelled: %w", ctx.Err())
			}
			state.Attempts++
			state.LastError = err.Error()
			if !isRetryableKeyserverError(err) {
				state.Permanent = true
				state.NextAttempt = nil
			} else if state.Attempts < keyserverMaxAttempts {
				next := time.Now().UTC().Add(keyserverRetryDelay(state.Attempts))
				state.NextAttempt = &next
			} else {
				state.NextAttempt = nil
			}
			states = putKeyserverState(states, index, state)
			errs = append(errs, fmt.Sprintf("%s: %v", ks, err))
			continue
		}
		state.Attempts++
		state.PublishedAt = time.Now().UTC()
		state.LastError = ""
		state.Permanent = false
		state.NextAttempt = nil
		states = putKeyserverState(states, index, state)
		published = append(published, state)
	}
	if len(errs) > 0 {
		return states, published, fmt.Errorf("keyserver publish failures: %s", strings.Join(errs, "; "))
	}
	return states, published, nil
}

func findKeyserverState(states []KeyserverState, fingerprint, keyserver string) int {
	for index, state := range states {
		if state.Fingerprint == fingerprint && state.URL == keyserver {
			return index
		}
	}
	return -1
}

func putKeyserverState(states []KeyserverState, index int, state KeyserverState) []KeyserverState {
	if index >= 0 {
		states[index] = state
		return states
	}
	return append(states, state)
}

func keyserverRetryDelay(attempt int) time.Duration {
	delay := keyserverRetryBackoff * time.Duration(1<<uint(max(0, attempt-1)))
	if delay > 6*time.Hour {
		return 6 * time.Hour
	}
	return delay
}

func isRetryableKeyserverError(err error) bool {
	var statusError *keyserverStatusError
	if errors.As(err, &statusError) {
		return statusError.status == http.StatusTooManyRequests || statusError.status >= http.StatusInternalServerError
	}
	return true
}

type keyserverStatusError struct {
	status int
	body   string
}

func (e *keyserverStatusError) Error() string {
	return fmt.Sprintf("keyserver returned %d: %s", e.status, e.body)
}

// submitToKeyserver POSTs the public key to a single keyserver. keys.openpgp.org
// (and Hagrid-based servers) accept a JSON body at /vks/v1/upload; classic HKP
// servers accept a urlencoded keytext form at /pks/add.
func submitToKeyserver(ctx context.Context, keyserver string, pubArmored []byte) error {
	keyserver = strings.TrimRight(keyserver, "/")
	urlValue := keyserver
	if !strings.Contains(keyserver, "://") {
		urlValue = "https://" + keyserver
	}
	base, err := url.Parse(urlValue)
	if err != nil {
		return fmt.Errorf("parse keyserver url: %w", err)
	}
	if base.Scheme != "https" || base.Host == "" || base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return fmt.Errorf("parse keyserver url: expected an HTTPS keyserver host")
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
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("read keyserver response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &keyserverStatusError{status: resp.StatusCode, body: strings.TrimSpace(string(body))}
	}
	if strings.EqualFold(base.Hostname(), "keys.openpgp.org") {
		var result struct {
			KeyFingerprint string `json:"key_fpr"`
			Status         string `json:"status"`
		}
		if err := json.Unmarshal(body, &result); err != nil || result.KeyFingerprint == "" || result.Status == "" {
			return fmt.Errorf("keyserver returned an invalid VKS success response")
		}
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
	if err := os.WriteFile(filepath.Join(dir, keyserverStateFile), data, 0o600); err != nil {
		return fmt.Errorf("write keyserver state: %w", err)
	}
	return nil
}

// LoadKeyserverState reads the persisted per-keyserver publish state. A missing
// file returns an empty slice with no error.
func LoadKeyserverState(configDir string) ([]KeyserverState, error) {
	data, err := os.ReadFile(filepath.Join(SecurityDir(configDir), keyserverStateFile))
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
func UpdateKeyserversPublished(db *database.AppDatabase, states []KeyserverState) error {
	data, err := json.Marshal(states)
	if err != nil {
		return fmt.Errorf("encode keyservers_published: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeQueryTimeout)
	defer cancel()
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE pgp_keypair SET keyservers_published = ?
		 WHERE id = (SELECT id FROM pgp_keypair ORDER BY id DESC LIMIT 1)`,
			string(data),
		)
		return err
	}); err != nil {
		return fmt.Errorf("update keyservers_published: %w", err)
	}
	return nil
}

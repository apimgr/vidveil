// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — researcher-supplied GPG key resolution.
package secreport

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// allowedKeyservers is an explicit allowlist of OpenPGP keyserver hostnames a
// researcher-supplied URL may point to. This avoids SSRF: fetching an
// arbitrary researcher-supplied URL from the server would otherwise let a
// submitter probe internal/private network addresses via the server's own
// egress. Only these well-known public keyservers are permitted.
var allowedKeyservers = map[string]bool{
	"keys.openpgp.org":     true,
	"keyserver.ubuntu.com": true,
	"pgp.mit.edu":          true,
	"keyserver.pgp.com":    true,
}

// ResolveResearcherKey accepts either a pasted ASCII-armored public key block
// or an https:// URL pointing at an allowlisted public keyserver, and returns
// the raw armored key bytes. Empty input returns nil, nil (no key supplied —
// the acknowledgment email falls back to plaintext).
func ResolveResearcherKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.Contains(raw, "BEGIN PGP PUBLIC KEY") {
		return []byte(raw), nil
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("researcher key must be a pasted PGP public key block or an https:// keyserver URL")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !allowedKeyservers[host] {
		return nil, fmt.Errorf("researcher key URL host %q is not an allowlisted keyserver", host)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		// Re-validate every redirect hop against the same allowlist — a
		// malicious keyserver response could otherwise redirect the
		// server's request to an internal address.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to non-https URL blocked")
			}
			redirectHost := strings.ToLower(strings.TrimSuffix(req.URL.Hostname(), "."))
			if !allowedKeyservers[redirectHost] {
				return fmt.Errorf("redirect to non-allowlisted host %q blocked", redirectHost)
			}
			return nil
		},
	}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return nil, fmt.Errorf("fetch researcher key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch researcher key: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read researcher key response: %w", err)
	}
	return body, nil
}

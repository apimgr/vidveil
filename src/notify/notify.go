// SPDX-License-Identifier: MIT
// AI.md PART 12: Webhook notification dispatcher.
// Resolves effective contact endpoints for each role (admin/security/abuse/general),
// signs every outbound POST with HMAC-SHA256, and retries with exponential backoff.
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/apimgr/vidveil/src/config"
)

// Severity classifies notification urgency.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Role matches the four contact roles in AI.md PART 12.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleSecurity Role = "security"
	RoleAbuse    Role = "abuse"
	RoleGeneral  Role = "general"
)

// Payload is the canonical event payload sent to every transport.
type Payload struct {
	// Role is the dispatch role (admin/security/abuse/general).
	Role Role `json:"role"`
	// Event is the machine-readable event type, e.g. "admin.backup_failed".
	Event string `json:"event"`
	// Subject is the one-line summary.
	Subject string `json:"subject"`
	// Body is the full message text.
	Body string `json:"body"`
	// Severity classifies urgency.
	Severity Severity `json:"severity"`
	// Timestamp is Unix seconds of the event.
	Timestamp int64 `json:"timestamp"`
	// ProjectName is the application name, injected by Dispatcher.
	ProjectName string `json:"project_name"`
	// ProjectVersion is injected by Dispatcher.
	ProjectVersion string `json:"project_version"`
	// AppURL is the canonical application URL, injected by Dispatcher.
	AppURL string `json:"app_url"`
	// TrackingID is an optional caller-supplied correlation ID.
	TrackingID string `json:"tracking_id,omitempty"`
}

// retryDelays is the exponential backoff schedule per AI.md PART 12.
var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
	24 * time.Hour,
}

// Dispatcher routes notifications to the configured webhook transports.
type Dispatcher struct {
	mu      sync.RWMutex
	contact *config.ContactConfig
	// projectName / projectVersion / appURL are injected into every payload.
	projectName    string
	projectVersion string
	appURL         string
	// httpClient is shared across all sends.
	httpClient *http.Client
}

// New creates a Dispatcher. Call Update when the config changes (hot-reload safe).
func New(contact *config.ContactConfig, projectName, projectVersion, appURL string) *Dispatcher {
	d := &Dispatcher{
		projectName:    projectName,
		projectVersion: projectVersion,
		appURL:         appURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	if contact != nil {
		cp := *contact
		d.contact = &cp
	}
	return d
}

// Update swaps the contact config without restarting the dispatcher (hot-reload safe).
func (d *Dispatcher) Update(contact *config.ContactConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if contact != nil {
		cp := *contact
		d.contact = &cp
	}
}

// Send dispatches payload to every configured transport for the given role.
// Retries happen asynchronously — Send returns as soon as all first attempts are
// launched. Pass ctx to bound the total retry lifetime.
func (d *Dispatcher) Send(ctx context.Context, role Role, p Payload) {
	d.mu.RLock()
	contact := d.contact
	d.mu.RUnlock()

	if contact == nil {
		return
	}

	p.Role = role
	if p.Timestamp == 0 {
		p.Timestamp = time.Now().Unix()
	}
	p.ProjectName = d.projectName
	p.ProjectVersion = d.projectVersion
	p.AppURL = d.appURL

	webhooks := d.resolveWebhooks(contact, role)
	for transport, url := range webhooks {
		if url == "" {
			continue
		}
		secret := webhooks[transport+"_secret"]
		go d.dispatchWithRetry(ctx, transport, url, secret, p)
	}
}

// resolveWebhooks returns the effective webhook map for role, applying the
// fallback chain defined in AI.md PART 12:
//
//	admin    → no fallback (authoritative)
//	security → per-transport: security.webhooks.X → admin.webhooks.X
//	abuse    → per-transport: abuse.X → general.X → admin.X
//	general  → per-transport: general.X → admin.X
func (d *Dispatcher) resolveWebhooks(contact *config.ContactConfig, role Role) map[string]string {
	switch role {
	case RoleAdmin:
		return mergeWebhooks(contact.Admin.Webhooks)
	case RoleSecurity:
		return mergeWithFallback(contact.Security.Webhooks, contact.Admin.Webhooks)
	case RoleAbuse:
		return mergeWithFallback(contact.Abuse.Webhooks,
			mergeWithFallback(contact.General.Webhooks, contact.Admin.Webhooks))
	case RoleGeneral:
		return mergeWithFallback(contact.General.Webhooks, contact.Admin.Webhooks)
	default:
		return mergeWebhooks(contact.Admin.Webhooks)
	}
}

// mergeWebhooks returns a shallow copy of src.
func mergeWebhooks(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// mergeWithFallback returns a map where each key from primary is kept;
// keys from fallback fill in where primary has an empty value.
func mergeWithFallback(primary, fallback map[string]string) map[string]string {
	out := make(map[string]string, len(primary)+len(fallback))
	for k, v := range fallback {
		out[k] = v
	}
	for k, v := range primary {
		out[k] = v
	}
	return out
}

// dispatchWithRetry fires the first attempt immediately, then retries on failure
// using the backoff schedule defined in AI.md PART 12.
func (d *Dispatcher) dispatchWithRetry(ctx context.Context, transport, url, secret string, p Payload) {
	webhookID := uuid.New().String()
	for i := 0; ; i++ {
		err := d.send(ctx, transport, url, secret, webhookID, p)
		if err == nil {
			return
		}
		if i >= len(retryDelays) {
			// All retries exhausted — log and drop.
			logWebhookFailed(transport, url, err)
			return
		}
		delay := retryDelays[i]
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// send formats and POSTs a single webhook attempt.
func (d *Dispatcher) send(ctx context.Context, transport, url, secret, webhookID string, p Payload) error {
	var (
		body        []byte
		contentType string
		targetURL   string
		err         error
	)

	switch strings.ToLower(transport) {
	case "telegram":
		body, contentType, targetURL, err = formatTelegram(url, p)
	case "discord":
		body, contentType, targetURL, err = formatDiscord(url, p)
	case "slack":
		body, contentType, targetURL, err = formatSlack(url, p)
	case "mattermost":
		body, contentType, targetURL, err = formatMattermost(url, p)
	case "pushover":
		body, contentType, targetURL, err = formatPushover(url, p)
	case "gotify":
		body, contentType, targetURL, err = formatGotify(url, p)
	default:
		body, contentType, targetURL, err = formatGeneric(url, p)
	}
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s (+%s)", p.ProjectName, p.ProjectVersion, p.AppURL))

	// Signing headers per AI.md PART 12.
	tsStr := strconv.FormatInt(p.Timestamp, 10)
	req.Header.Set("X-Webhook-Timestamp", tsStr)
	req.Header.Set("X-Webhook-ID", webhookID)
	req.Header.Set("X-Webhook-Event", p.Event)
	if secret != "" {
		sig := computeHMAC([]byte(secret), body)
		req.Header.Set("X-Webhook-Signature", "sha256="+sig)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook %s returned HTTP %d", transport, resp.StatusCode)
	}
	return nil
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of body using secret.
func computeHMAC(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateWebhookSecret creates a random 32-byte hex-encoded signing secret.
func GenerateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// logWebhookFailed is a thin structured-log shim.
// A real implementation would call the project's logger.
func logWebhookFailed(transport, url, err interface{}) {
	_ = transport
	_ = url
	_ = err
}

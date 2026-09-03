// SPDX-License-Identifier: MIT
package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// ---- helpers ----

func testPayload() Payload {
	return Payload{
		Role:      RoleAdmin,
		Event:     "admin.test",
		Subject:   "Test notification",
		Body:      "This is the test body.",
		Severity:  SeverityInfo,
		Timestamp: 1700000000,
	}
}

func testContact(url string) *config.ContactConfig {
	return &config.ContactConfig{
		Admin: config.ContactRoleConfig{
			Email:    "admin@example.com",
			Webhooks: map[string]string{"generic": url},
		},
		Security: config.ContactRoleConfig{
			Email:    "security@example.com",
			Webhooks: map[string]string{},
		},
		Abuse: config.ContactRoleConfig{
			Email:    "",
			Webhooks: map[string]string{},
		},
		General: config.ContactRoleConfig{
			Email:    "",
			Webhooks: map[string]string{},
		},
	}
}

// ---- Dispatcher ----

func TestNewDispatcher(t *testing.T) {
	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	if d == nil {
		t.Fatal("New returned nil")
	}
}

func TestDispatcherUpdate(t *testing.T) {
	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	c := testContact("https://example.com/webhook")
	d.Update(c)
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.contact == nil {
		t.Error("Update did not set contact")
	}
}

func TestDispatcherSendNilContact(t *testing.T) {
	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	// Must not panic when contact is nil.
	d.Send(context.Background(), RoleAdmin, testPayload())
}

// TestDispatcherSendReachesServer verifies the dispatcher POSTs to the webhook URL.
func TestDispatcherSendReachesServer(t *testing.T) {
	received := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	d := New(testContact(ts.URL), "vidveil", "1.0.0", ts.URL)
	d.Send(context.Background(), RoleAdmin, testPayload())

	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Error("webhook server was never reached within 3s")
	}
}

// ---- computeHMAC ----

func TestComputeHMAC(t *testing.T) {
	secret := []byte("test-secret")
	body := []byte(`{"event":"test"}`)
	got := computeHMAC(secret, body)

	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Errorf("computeHMAC = %s, want %s", got, want)
	}
}

// ---- GenerateWebhookSecret ----

func TestGenerateWebhookSecret(t *testing.T) {
	s1, err := GenerateWebhookSecret()
	if err != nil {
		t.Fatalf("GenerateWebhookSecret: %v", err)
	}
	if len(s1) != 64 {
		t.Errorf("GenerateWebhookSecret: expected 64 hex chars, got %d", len(s1))
	}
	s2, _ := GenerateWebhookSecret()
	if s1 == s2 {
		t.Error("GenerateWebhookSecret returned identical values on consecutive calls")
	}
}

// ---- mergeWithFallback ----

func TestMergeWithFallback(t *testing.T) {
	primary := map[string]string{"a": "primary-a", "b": ""}
	fallback := map[string]string{"a": "fallback-a", "b": "fallback-b", "c": "fallback-c"}
	got := mergeWithFallback(primary, fallback)
	// primary wins when non-empty
	if got["a"] != "primary-a" {
		t.Errorf("a: got %q, want %q", got["a"], "primary-a")
	}
	// primary empty string stays (does not defer to fallback via this function)
	if got["c"] != "fallback-c" {
		t.Errorf("c: got %q, want %q", got["c"], "fallback-c")
	}
}

// ---- resolveWebhooks ----

func TestResolveWebhooksAdmin(t *testing.T) {
	contact := testContact("https://example.com/wh")
	d := New(contact, "vidveil", "1.0.0", "https://example.com")
	m := d.resolveWebhooks(contact, RoleAdmin)
	if m["generic"] != "https://example.com/wh" {
		t.Errorf("admin webhook not resolved: %v", m)
	}
}

func TestResolveWebhooksSecurityFallsBackToAdmin(t *testing.T) {
	contact := &config.ContactConfig{
		Admin: config.ContactRoleConfig{
			Email:    "admin@example.com",
			Webhooks: map[string]string{"discord": "https://discord.com/wh"},
		},
		Security: config.ContactRoleConfig{
			Email:    "security@example.com",
			Webhooks: map[string]string{},
		},
	}
	d := New(contact, "vidveil", "1.0.0", "https://example.com")
	m := d.resolveWebhooks(contact, RoleSecurity)
	if m["discord"] != "https://discord.com/wh" {
		t.Errorf("security did not fall back to admin discord: %v", m)
	}
}

// ---- Adapter format functions ----

func TestFormatGenericReturnsJSON(t *testing.T) {
	p := testPayload()
	body, ct, url, err := formatGeneric("https://example.com/hook", p)
	if err != nil {
		t.Fatalf("formatGeneric error: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	if url != "https://example.com/hook" {
		t.Errorf("url = %q, want https://example.com/hook", url)
	}
	var out Payload
	if err := json.Unmarshal(body, &out); err != nil {
		t.Errorf("body is not valid JSON: %v", err)
	}
	if out.Event != p.Event {
		t.Errorf("event = %q, want %q", out.Event, p.Event)
	}
}

func TestFormatDiscordContainsContent(t *testing.T) {
	p := testPayload()
	body, ct, _, err := formatDiscord("https://discord.com/api/webhooks/123/abc", p)
	if err != nil {
		t.Fatalf("formatDiscord: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var out map[string]interface{}
	json.Unmarshal(body, &out)
	if _, ok := out["content"]; !ok {
		t.Error("discord body missing 'content' key")
	}
}

func TestFormatSlackContainsText(t *testing.T) {
	p := testPayload()
	body, _, _, err := formatSlack("https://hooks.slack.com/services/T/B/X", p)
	if err != nil {
		t.Fatalf("formatSlack: %v", err)
	}
	var out map[string]interface{}
	json.Unmarshal(body, &out)
	if _, ok := out["text"]; !ok {
		t.Error("slack body missing 'text' key")
	}
}

func TestFormatMattermostContainsText(t *testing.T) {
	p := testPayload()
	body, _, _, err := formatMattermost("https://mattermost.example.com/hooks/abc", p)
	if err != nil {
		t.Fatalf("formatMattermost: %v", err)
	}
	var out map[string]interface{}
	json.Unmarshal(body, &out)
	if _, ok := out["text"]; !ok {
		t.Error("mattermost body missing 'text' key")
	}
}

func TestFormatTelegramContainsText(t *testing.T) {
	p := testPayload()
	body, ct, url, err := formatTelegram("https://api.telegram.org/bot123/sendMessage?chat_id=456", p)
	if err != nil {
		t.Fatalf("formatTelegram: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	// Per AI.md PART 12 the message rides in the query with an empty body.
	if len(body) != 0 {
		t.Errorf("telegram body should be empty, got %q", body)
	}
	if !strings.Contains(url, "text=") {
		t.Errorf("telegram URL does not contain text param: %q", url)
	}
	if !strings.Contains(url, "chat_id=456") {
		t.Errorf("telegram URL dropped chat_id: %q", url)
	}
}

func TestFormatGotifyBuildsMessagePath(t *testing.T) {
	p := testPayload()
	body, ct, targetURL, err := formatGotify("https://gotify.example.com?token=abc123", p)
	if err != nil {
		t.Fatalf("formatGotify: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	if !strings.Contains(targetURL, "/message") {
		t.Errorf("gotify URL missing /message path: %q", targetURL)
	}
	if !strings.Contains(targetURL, "token=abc123") {
		t.Errorf("gotify URL missing token param: %q", targetURL)
	}
	var out map[string]interface{}
	json.Unmarshal(body, &out)
	if _, ok := out["title"]; !ok {
		t.Error("gotify body missing 'title' key")
	}
	if _, ok := out["message"]; !ok {
		t.Error("gotify body missing 'message' key")
	}
}

func TestFormatPushoverContainsMessage(t *testing.T) {
	p := testPayload()
	body, ct, targetURL, err := formatPushover("https://api.pushover.net/1/messages.json?token=tok123&user=usr456", p)
	if err != nil {
		t.Fatalf("formatPushover: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	// Credentials must be stripped from the target URL and carried in the body.
	if strings.Contains(targetURL, "token") || strings.Contains(targetURL, "user") {
		t.Errorf("pushover target URL still carries credentials: %q", targetURL)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("pushover body not valid JSON: %v", err)
	}
	if out["token"] != "tok123" {
		t.Errorf("pushover body token = %v, want tok123", out["token"])
	}
	if out["user"] != "usr456" {
		t.Errorf("pushover body user = %v, want usr456", out["user"])
	}
	if _, ok := out["message"]; !ok {
		t.Error("pushover body missing 'message' key")
	}
}

// TestFormatPushoverMissingCredentials verifies the adapter rejects a URL that
// lacks token/user rather than silently sending empty credentials.
func TestFormatPushoverMissingCredentials(t *testing.T) {
	p := testPayload()
	if _, _, _, err := formatPushover("https://api.pushover.net/1/messages.json", p); err == nil {
		t.Error("formatPushover: expected error when token/user missing, got nil")
	}
}

// ---- resolveWebhooks additional roles ----

func TestResolveWebhooksAbuseFallsBackToGeneralThenAdmin(t *testing.T) {
	contact := &config.ContactConfig{
		Admin: config.ContactRoleConfig{
			Webhooks: map[string]string{"slack": "https://admin.slack.com/wh"},
		},
		General: config.ContactRoleConfig{
			Webhooks: map[string]string{"discord": "https://general.discord.com/wh"},
		},
		Abuse: config.ContactRoleConfig{
			Webhooks: map[string]string{},
		},
	}
	d := New(contact, "vidveil", "1.0.0", "https://example.com")
	m := d.resolveWebhooks(contact, RoleAbuse)
	if m["slack"] != "https://admin.slack.com/wh" {
		t.Errorf("abuse did not fall back to admin slack: %v", m)
	}
	if m["discord"] != "https://general.discord.com/wh" {
		t.Errorf("abuse did not inherit general discord: %v", m)
	}
}

func TestResolveWebhooksGeneralFallsBackToAdmin(t *testing.T) {
	contact := &config.ContactConfig{
		Admin: config.ContactRoleConfig{
			Webhooks: map[string]string{"gotify": "https://gotify.example.com?token=x"},
		},
		General: config.ContactRoleConfig{
			Webhooks: map[string]string{},
		},
	}
	d := New(contact, "vidveil", "1.0.0", "https://example.com")
	m := d.resolveWebhooks(contact, RoleGeneral)
	if m["gotify"] != "https://gotify.example.com?token=x" {
		t.Errorf("general did not fall back to admin gotify: %v", m)
	}
}

func TestResolveWebhooksUnknownRoleFallsBackToAdmin(t *testing.T) {
	contact := testContact("https://admin.example.com/wh")
	d := New(contact, "vidveil", "1.0.0", "https://example.com")
	m := d.resolveWebhooks(contact, Role("unknown"))
	if m["generic"] != "https://admin.example.com/wh" {
		t.Errorf("unknown role did not fall back to admin: %v", m)
	}
}

// ---- send error paths ----

func TestSendHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.httpClient = ts.Client()

	err := d.send(context.Background(), "generic", ts.URL, "", "uuid-999", testPayload())
	if err == nil {
		t.Error("send: expected error on HTTP 500, got nil")
	}
}

func TestSendNetworkError(t *testing.T) {
	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	err := d.send(context.Background(), "generic", "http://127.0.0.1:1", "", "uuid-net", testPayload())
	if err == nil {
		t.Error("send: expected network error on unreachable address, got nil")
	}
}

// ---- dispatchWithRetry ----

func TestDispatchWithRetryExhaustsAllRetries(t *testing.T) {
	// Use a server that always returns 500.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.httpClient = ts.Client()

	// Override retry delays to zero so the test completes quickly.
	original := retryDelays
	retryDelays = []time.Duration{0, 0}
	defer func() { retryDelays = original }()

	done := make(chan struct{})
	go func() {
		d.dispatchWithRetry(context.Background(), "generic", ts.URL, "", testPayload())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("dispatchWithRetry did not finish within 5s")
	}
}

func TestDispatchWithRetryContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.httpClient = ts.Client()

	// Use a long retry delay so context cancellation is the exit path.
	original := retryDelays
	retryDelays = []time.Duration{5 * time.Minute}
	defer func() { retryDelays = original }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.dispatchWithRetry(ctx, "generic", ts.URL, "", testPayload())
		close(done)
	}()

	// Cancel after a short delay to trigger ctx.Done() path.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("dispatchWithRetry did not exit on context cancellation within 3s")
	}
}

// ---- logWebhookFailed ----

// TestLogWebhookFailed_NoSinkIsNoop verifies logging is a no-op (no panic) when
// no failure sink is configured.
func TestLogWebhookFailed_NoSinkIsNoop(t *testing.T) {
	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.logWebhookFailed("telegram", "https://api.telegram.org/botX?chat_id=1", fmt.Errorf("connection refused"))
}

// TestLogWebhookFailed_MasksAndRateLimits verifies the sink is invoked with the
// query string masked and is rate-limited per target URL.
func TestLogWebhookFailed_MasksAndRateLimits(t *testing.T) {
	var mu sync.Mutex
	var calls []map[string]any
	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.SetFailureLogger(func(event string, fields map[string]any) {
		mu.Lock()
		defer mu.Unlock()
		if event != "notify.webhook_failed" {
			t.Errorf("event = %q, want notify.webhook_failed", event)
		}
		calls = append(calls, fields)
	})

	rawURL := "https://api.telegram.org/botTOKEN/sendMessage?chat_id=SECRET&text=hi"
	d.logWebhookFailed("telegram", rawURL, fmt.Errorf("boom"))
	d.logWebhookFailed("telegram", rawURL, fmt.Errorf("boom again"))

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected 1 logged failure (rate-limited), got %d", len(calls))
	}
	got, _ := calls[0]["url"].(string)
	if strings.Contains(got, "SECRET") || strings.Contains(got, "chat_id") {
		t.Errorf("masked url still leaks query: %q", got)
	}
	if !strings.Contains(got, "xxxxx") {
		t.Errorf("masked url = %q, want masked query", got)
	}
}

// TestMaskURL checks the query string is masked and credentials removed.
func TestMaskURL(t *testing.T) {
	got := maskURL("https://user:pass@host/path?token=abc")
	if strings.Contains(got, "token=abc") || strings.Contains(got, "pass") {
		t.Errorf("maskURL leaked credentials: %q", got)
	}
	if !strings.Contains(got, "host/path") {
		t.Errorf("maskURL dropped host/path: %q", got)
	}
}

// TestDispatchWithRetry_InvokesFailureLogger verifies that once all retries are
// exhausted the configured failure sink receives one entry.
func TestDispatchWithRetry_InvokesFailureLogger(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.httpClient = ts.Client()

	logged := make(chan map[string]any, 1)
	d.SetFailureLogger(func(event string, fields map[string]any) {
		select {
		case logged <- fields:
		default:
		}
	})

	original := retryDelays
	retryDelays = []time.Duration{0, 0}
	defer func() { retryDelays = original }()

	go d.dispatchWithRetry(context.Background(), "generic", ts.URL, "", testPayload())

	select {
	case fields := <-logged:
		if fields["transport"] != "generic" {
			t.Errorf("transport = %v, want generic", fields["transport"])
		}
	case <-time.After(5 * time.Second):
		t.Error("failure logger was never invoked")
	}
}

// ---- GenerateWebhookSecret error path ----

func TestGenerateWebhookSecretUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		s, err := GenerateWebhookSecret()
		if err != nil {
			t.Fatalf("GenerateWebhookSecret: %v", err)
		}
		if seen[s] {
			t.Fatalf("GenerateWebhookSecret returned duplicate value: %s", s)
		}
		seen[s] = true
	}
}

// ---- Signature header ----

func TestSendIncludesSignatureHeader(t *testing.T) {
	var gotSig string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.httpClient = ts.Client()

	p := testPayload()
	err := d.send(context.Background(), "generic", ts.URL, "mysecret", "uuid-123", p)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("X-Webhook-Signature = %q, want sha256= prefix", gotSig)
	}
}

func TestSendNoSecretOmitsSignature(t *testing.T) {
	var gotSig string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	d := New(nil, "vidveil", "1.0.0", "https://example.com")
	d.httpClient = ts.Client()

	p := testPayload()
	err := d.send(context.Background(), "generic", ts.URL, "", "uuid-123", p)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotSig != "" {
		t.Errorf("X-Webhook-Signature should be empty when no secret, got %q", gotSig)
	}
}

// ── Timestamp zero branch (lines 132–133) ────────────────────────────────────

// Send with Timestamp=0 triggers the auto-fill branch that sets it to time.Now().Unix().
func TestSend_TimestampZero_AutoFills(t *testing.T) {
	received := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	d := New(testContact(ts.URL), "vidveil", "1.0.0", ts.URL)
	p := testPayload()
	p.Timestamp = 0
	d.Send(context.Background(), RoleAdmin, p)

	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Error("TimestampZero: webhook server never reached")
	}
}

// ── Empty webhook URL skipped (lines 141–142) ─────────────────────────────────

// Send with an empty webhook URL skips dispatch without panicking.
func TestSend_EmptyWebhookURL_SkipsDispatch(t *testing.T) {
	d := New(testContact(""), "vidveil", "1.0.0", "https://example.com")
	d.Send(context.Background(), RoleAdmin, testPayload())
	// Must not panic; no network call is made.
}

// ── dispatchWebhook switch cases (lines 227–238) ─────────────────────────────

// TestSend_WebhookTransports_CoversSwitchCases exercises each named transport case
// in the dispatchWebhook switch (telegram, discord, slack, mattermost, pushover, gotify).
func TestSend_WebhookTransports_CoversSwitchCases(t *testing.T) {
	transports := []string{"telegram", "discord", "slack", "mattermost", "pushover", "gotify"}
	for _, transport := range transports {
		t.Run(transport, func(t *testing.T) {
			received := make(chan struct{}, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case received <- struct{}{}:
				default:
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			d := New(nil, "vidveil", "1.0.0", "https://example.com")
			d.httpClient = srv.Client()

			// Pushover requires token/user query params on its URL.
			targetURL := srv.URL
			if transport == "pushover" {
				targetURL = srv.URL + "?token=tok&user=usr"
			}

			err := d.send(context.Background(), transport, targetURL, "", "uuid-"+transport, testPayload())
			if err != nil {
				t.Errorf("send %s: %v", transport, err)
			}

			select {
			case <-received:
			case <-time.After(3 * time.Second):
				t.Errorf("%s: server was never reached", transport)
			}
		})
	}
}

// SPDX-License-Identifier: MIT
package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if !strings.Contains(url, "text=") {
		t.Errorf("telegram URL does not contain text param: %q", url)
	}
	var out map[string]interface{}
	json.Unmarshal(body, &out)
	if _, ok := out["text"]; !ok {
		t.Error("telegram body missing 'text' key")
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
	body, ct, _, err := formatPushover("https://api.pushover.net/1/messages.json", p)
	if err != nil {
		t.Fatalf("formatPushover: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var out map[string]interface{}
	json.Unmarshal(body, &out)
	if _, ok := out["message"]; !ok {
		t.Error("pushover body missing 'message' key")
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

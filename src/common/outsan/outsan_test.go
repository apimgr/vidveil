// SPDX-License-Identifier: MIT
package outsan

import (
	"strings"
	"testing"
)

func TestSanitize_RedactsSensitiveQueryParams(t *testing.T) {
	in := map[string]interface{}{
		"url": "https://example.com/reset?token=abc123&user=bob",
	}
	out := Sanitize(in, false).(map[string]interface{})
	got := out["url"].(string)
	if strings.Contains(got, "abc123") {
		t.Fatalf("expected token redacted, got %q", got)
	}
	if !strings.Contains(got, "user=bob") {
		t.Fatalf("expected non-sensitive param preserved, got %q", got)
	}
}

func TestSanitize_StripsInternalIP(t *testing.T) {
	in := map[string]interface{}{
		"message": "connection failed to 10.0.0.5 during handshake",
	}
	out := Sanitize(in, false).(map[string]interface{})
	got := out["message"].(string)
	if strings.Contains(got, "10.0.0.5") {
		t.Fatalf("expected internal IP redacted, got %q", got)
	}
}

func TestSanitize_StripsInternalPath(t *testing.T) {
	in := map[string]interface{}{
		"message": "failed to open /etc/vidveil/server.yml: permission denied",
	}
	out := Sanitize(in, false).(map[string]interface{})
	got := out["message"].(string)
	if strings.Contains(got, "/etc/vidveil/server.yml") {
		t.Fatalf("expected filesystem path redacted, got %q", got)
	}
}

func TestSanitize_TruncatesMessageField(t *testing.T) {
	in := map[string]interface{}{
		"message": strings.Repeat("a", 500),
	}
	out := Sanitize(in, false).(map[string]interface{})
	got := out["message"].(string)
	if len([]rune(got)) > messageFieldLimit+1 {
		t.Fatalf("expected message truncated to %d runes, got %d", messageFieldLimit, len([]rune(got)))
	}
}

func TestSanitize_DoesNotTruncateOrdinaryLongContent(t *testing.T) {
	longTitle := strings.Repeat("b", 500)
	in := map[string]interface{}{
		"title": longTitle,
	}
	out := Sanitize(in, false).(map[string]interface{})
	got := out["title"].(string)
	if got != longTitle {
		t.Fatalf("expected ordinary long field untouched, got len %d want %d", len(got), len(longTitle))
	}
}

func TestSanitize_StripsDevOnlyFieldsInProduction(t *testing.T) {
	in := map[string]interface{}{
		"ok":     true,
		"_debug": map[string]interface{}{"stack": "trace here"},
	}
	out := Sanitize(in, true).(map[string]interface{})
	if _, ok := out["_debug"]; ok {
		t.Fatalf("expected _debug stripped in production, got %v", out)
	}
	out = Sanitize(in, false).(map[string]interface{})
	if _, ok := out["_debug"]; !ok {
		t.Fatalf("expected _debug preserved in dev mode, got %v", out)
	}
}

func TestSanitize_RecursesIntoArrays(t *testing.T) {
	in := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"url": "https://x/y?token=secret1"},
			map[string]interface{}{"url": "https://x/y?token=secret2"},
		},
	}
	out := Sanitize(in, false).(map[string]interface{})
	items := out["data"].([]interface{})
	for _, item := range items {
		u := item.(map[string]interface{})["url"].(string)
		if strings.Contains(u, "secret1") || strings.Contains(u, "secret2") {
			t.Fatalf("expected token redacted inside array element, got %q", u)
		}
	}
}

// SPDX-License-Identifier: MIT
package logging

import (
	"strings"
	"testing"
)

func TestSanitizeLogFieldsRedactsCredentialVariantsAndURLs(t *testing.T) {
	fields := map[string]interface{}{
		"session_token": "session-secret",
		"Authorization": "Bearer header-secret",
		"request_url":   "https://example.test/callback?access_token=query-secret&state=ok",
		"nested": map[string]interface{}{
			"database.password": "db-secret",
		},
	}
	cleaned := SanitizeLogFields(fields)
	for _, value := range []string{
		"session-secret", "header-secret", "query-secret", "db-secret",
	} {
		if strings.Contains(strings.Join([]string{
			cleaned["session_token"].(string),
			cleaned["Authorization"].(string),
			cleaned["request_url"].(string),
			cleaned["nested"].(map[string]interface{})["database.password"].(string),
		}, " "), value) {
			t.Fatalf("sensitive value %q survived log sanitization: %#v", value, cleaned)
		}
	}
}

func TestSanitizeLogTextRedactsCredentialAssignments(t *testing.T) {
	message := sanitizeLogText("request failed access_token=inline-secret url=https://example.test/?token=query-secret")
	if strings.Contains(message, "inline-secret") || strings.Contains(message, "query-secret") {
		t.Fatalf("sensitive credential survived log message sanitization: %q", message)
	}
}

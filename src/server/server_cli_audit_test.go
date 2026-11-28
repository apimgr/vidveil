// SPDX-License-Identifier: MIT
package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestRejectedCLIAuthenticationIsAuditedWithoutToken(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	s := newTestServerWithOptions(t, func(cfg *config.AppConfig) {
		cfg.CLI.BinaryDownload.RequireAuth = true
		cfg.Server.Logs.Audit.Enabled = true
		cfg.Server.Logs.Audit.Filename = auditPath
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/cli/binaries/vidveil-cli-linux-amd64", nil)
	request.Header.Set("User-Agent", "vidveil-cli/1.2.3")
	request.Header.Set("Authorization", "Bearer rejected-secret-token")
	response := httptest.NewRecorder()
	s.requireOperatorToken(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("rejected CLI request reached protected handler")
	})).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("rejected CLI status = %d, want 401", response.Code)
	}
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	var entry struct {
		Event   string                 `json:"event"`
		Details map[string]interface{} `json:"details"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("parse audit event: %v", err)
	}
	if entry.Event != "cli.token_rejected" {
		t.Fatalf("audit event = %q, want cli.token_rejected", entry.Event)
	}
	prefixHash := sha256.Sum256([]byte("rejected-sec"))
	wantPrefixHash := hex.EncodeToString(prefixHash[:4])
	if entry.Details["token_prefix_sha256"] != wantPrefixHash {
		t.Fatalf("audited token prefix hash = %v, want %s", entry.Details["token_prefix_sha256"], wantPrefixHash)
	}
	if strings.Contains(string(data), "rejected-secret-token") {
		t.Fatal("audit log contains the rejected token")
	}
}

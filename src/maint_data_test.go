// SPDX-License-Identifier: MIT
package main

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/database"
)

func TestReadTableForExportUsesCurrentSchema(t *testing.T) {
	mgr, err := database.NewSchemaManager(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatalf("NewSchemaManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	for _, table := range exportableTables {
		t.Run(table, func(t *testing.T) {
			if _, err := readTableForExport(mgr.GetDB(), table); err != nil {
				t.Fatalf("readTableForExport: %v", err)
			}
		})
	}
}

func TestNewDataRequestIDIsUniqueHex(t *testing.T) {
	first, err := newDataRequestID()
	if err != nil {
		t.Fatalf("newDataRequestID: %v", err)
	}
	second, err := newDataRequestID()
	if err != nil {
		t.Fatalf("newDataRequestID: %v", err)
	}
	if len(first) != 32 {
		t.Errorf("request ID length = %d, want 32 hex characters", len(first))
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Errorf("request ID %q is not hex: %v", first, err)
	}
	if first == second {
		t.Error("consecutive request IDs are equal")
	}
}

func TestAuditLogUserIDAnonymizedWithMarker(t *testing.T) {
	for _, rule := range anonymizations {
		if rule.table == "audit_log" && rule.column == "user_id" {
			if rule.replacement != redactedMarker {
				t.Errorf("audit_log.user_id replacement = %q, want %q", rule.replacement, redactedMarker)
			}
			return
		}
	}
	t.Fatal("missing audit_log.user_id anonymization rule")
}

func TestExportTableQueryOmitsSensitiveColumns(t *testing.T) {
	for _, table := range exportableTables {
		query, ok := exportTableQuery(table)
		if !ok {
			t.Fatalf("exportTableQuery(%q) missing query", table)
		}
		if strings.Contains(query, "*") {
			t.Errorf("exportTableQuery(%q) uses wildcard columns", table)
		}
	}
	for _, table := range []string{"security_reports", "api_tokens"} {
		query, _ := exportTableQuery(table)
		for _, secretColumn := range []string{"encrypted_body", "token_hash", "token_enc"} {
			if strings.Contains(query, secretColumn) {
				t.Errorf("exportTableQuery(%q) includes sensitive column %q", table, secretColumn)
			}
		}
	}
}

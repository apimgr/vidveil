// SPDX-License-Identifier: MIT
package logging

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestGenerateAuditIDUsesULIDFormat(t *testing.T) {
	id := generateAuditID()
	if matched, _ := regexp.MatchString(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`, id); !matched {
		t.Fatalf("audit ID %q is not a ULID", id)
	}
}

func TestInspectAuditChainVerifiesLineHashes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	anchor := filepath.Join(filepath.Dir(path), "audit.anchor")
	first, _ := json.Marshal(AuditEntry{ID: generateAuditID(), Prev: zeroAuditHash})
	second, _ := json.Marshal(AuditEntry{ID: generateAuditID(), Prev: auditLineHash(first)})
	content := append(append(append([]byte(nil), first...), '\n'), second...)
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	last, issue := inspectAuditChain(path, anchor)
	if issue != nil || last != auditLineHash(second) {
		t.Fatalf("inspect valid chain = (%q, %#v)", last, issue)
	}
	broken, _ := json.Marshal(AuditEntry{ID: generateAuditID(), Prev: zeroAuditHash})
	content = append(append(append([]byte(nil), first...), '\n'), append(broken, '\n')...)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	_, issue = inspectAuditChain(path, anchor)
	if issue == nil || issue.reason != "chain_mismatch" || issue.line != 2 {
		t.Fatalf("inspect broken chain = %#v, want line 2 chain_mismatch", issue)
	}
}

func TestAuditLogIntegrityReportsCountAndBrokenLocation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	anchor := filepath.Join(dir, "audit.anchor")
	entry, _ := json.Marshal(AuditEntry{ID: generateAuditID(), Prev: zeroAuditHash})
	if err := os.WriteFile(path, append(entry, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if got := AuditLogIntegrity(path, anchor); got != "intact (1 entries)" {
		t.Fatalf("AuditLogIntegrity(valid) = %q", got)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := AuditLogIntegrity(path, anchor); got != "broken at audit.log:1" {
		t.Fatalf("AuditLogIntegrity(broken) = %q", got)
	}
}

func TestWriteAuditAnchorReplacesSecurely(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.anchor")
	first := auditLineHash([]byte("first"))
	second := auditLineHash([]byte("second"))
	if err := writeAuditAnchor(path, first); err != nil {
		t.Fatal(err)
	}
	if err := writeAuditAnchor(path, second); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != second+"\n" {
		t.Fatalf("anchor = %q, want %q", contents, second+"\n")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("anchor mode = %o, want 0600", got)
	}
}

func TestAuditWritesAndMirrorsOnlyAfterSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	writer, err := NewRotatingFile(path, RotationConfig{FileMode: 0600})
	if err != nil {
		t.Fatal(err)
	}
	logger := &AppLogger{
		outputs:       map[string]io.Writer{"audit": writer},
		outputFormats: map[string]string{},
		auditPrevious: zeroAuditHash,
	}
	var mirrored AuditEntry
	logger.SetAuditMirror(func(entry AuditEntry) error {
		mirrored = entry
		return nil
	})
	logger.Audit("config.updated", "operator-user", "admin", "192.0.2.1", "success", map[string]interface{}{"changed_keys": []string{"server.port"}, "source": "api"})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	line, err := bufio.NewReader(file).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var stored AuditEntry
	if err := json.Unmarshal(line, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.ID == "" || stored.Prev != zeroAuditHash || stored.Details["audit_id"] != stored.ID || stored.Source != "api" {
		t.Fatalf("stored audit entry = %#v", stored)
	}
	if mirrored.ID != stored.ID || mirrored.Details["audit_id"] != stored.ID || mirrored.Actor.Type != "operator" {
		t.Fatalf("mirrored entry = %#v", mirrored)
	}
}

func TestAuditKeepsOwnerIDAndOmitsSystemIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	writer, err := NewRotatingFile(path, RotationConfig{FileMode: 0600})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	logger := &AppLogger{outputs: map[string]io.Writer{"audit": writer}}
	logger.Audit("token.created", "owner-12345678", "owner", "", "success", nil)
	logger.Audit("server.started", "ignored", "system", "192.0.2.1", "success", nil)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	entries := make([]AuditEntry, 0, 2)
	for scanner.Scan() {
		var entry AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Actor.Type != "owner" || entries[0].Actor.ID != "owner-12345678" {
		t.Fatalf("owner identity = %#v", entries)
	}
	if entries[1].Actor.Type != "system" || entries[1].Actor.ID != "" || entries[1].Actor.IP != "" {
		t.Fatalf("system identity = %#v", entries[1].Actor)
	}
}

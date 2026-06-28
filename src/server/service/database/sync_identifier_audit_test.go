// SPDX-License-Identifier: MIT
// Tests for the SQL identifier validation added during the project audit.
package database

import "testing"

// TestValidIdentifier verifies safe SQL identifiers are accepted and unsafe
// ones (used to interpolate column names in sync events) are rejected.
func TestValidIdentifier(t *testing.T) {
	valid := []string{"id", "user_id", "Name", "_internal", "col123"}
	for _, s := range valid {
		if !validIdentifier(s) {
			t.Errorf("validIdentifier(%q) = false, want true", s)
		}
	}
	invalid := []string{
		"",
		"1col",
		"user id",
		"name; DROP TABLE users",
		"col-name",
		"a.b",
		"col)",
		"\"quoted\"",
	}
	for _, s := range invalid {
		if validIdentifier(s) {
			t.Errorf("validIdentifier(%q) = true, want false", s)
		}
	}
}

// TestApplyInsertRejectsBadColumn verifies an INSERT sync event with an
// injection-style column name is rejected before reaching the database.
func TestApplyInsertRejectsBadColumn(t *testing.T) {
	sm := &SyncManager{}
	event := &SyncEvent{
		Type:  SyncEventInsert,
		Table: "users",
		Data:  map[string]interface{}{"name); DROP TABLE users; --": "x"},
	}
	if err := sm.applyInsert(event); err == nil {
		t.Error("applyInsert with malicious column should return an error")
	}
}

// TestApplyUpdateRejectsBadColumn verifies an UPDATE sync event with an
// injection-style column name is rejected before reaching the database.
func TestApplyUpdateRejectsBadColumn(t *testing.T) {
	sm := &SyncManager{}
	event := &SyncEvent{
		Type:       SyncEventUpdate,
		Table:      "users",
		PrimaryKey: 1,
		Data:       map[string]interface{}{"name = 'x'; --": "y"},
	}
	if err := sm.applyUpdate(event); err == nil {
		t.Error("applyUpdate with malicious column should return an error")
	}
}

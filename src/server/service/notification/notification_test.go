// SPDX-License-Identifier: MIT
// AI.md PART 28: Unit tests for notification service

package notification

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpDir := filepath.Join(os.TempDir(), "apimgr", "vidveil-test-"+t.Name())
	os.MkdirAll(tmpDir, 0755)
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestNewService(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.db != db {
		t.Error("Service db not set correctly")
	}
}

func TestEnsureSchema(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()

	if err := svc.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// Verify table exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='notifications'").Scan(&count)
	if err != nil {
		t.Fatalf("check table: %v", err)
	}
	if count != 1 {
		t.Error("notifications table not created")
	}
}

func TestSendAndGetUnread(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Send notification targeting center
	notif := &Notification{
		Type:    TypeInfo,
		Title:   "Test notification",
		Message: "Test message",
		Targets: TargetCenter,
	}

	if err := svc.Send(ctx, notif); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Should have ID now
	if notif.ID == "" {
		t.Error("notification ID not set")
	}

	// Get unread
	unread, err := svc.GetUnread(ctx, 10)
	if err != nil {
		t.Fatalf("GetUnread: %v", err)
	}

	if len(unread) != 1 {
		t.Fatalf("expected 1 unread, got %d", len(unread))
	}

	if unread[0].Title != "Test notification" {
		t.Errorf("title = %q, want %q", unread[0].Title, "Test notification")
	}
}

func TestSendToastOnly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Send toast-only notification (not stored in DB)
	notif := &Notification{
		Type:    TypeSuccess,
		Title:   "Toast only",
		Targets: TargetToast,
	}

	if err := svc.Send(ctx, notif); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Should NOT be in DB
	unread, err := svc.GetUnread(ctx, 10)
	if err != nil {
		t.Fatalf("GetUnread: %v", err)
	}

	if len(unread) != 0 {
		t.Errorf("toast-only should not be stored, got %d", len(unread))
	}
}

func TestGetUnreadCount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Send 3 notifications
	for i := 0; i < 3; i++ {
		svc.Send(ctx, &Notification{
			Type:    TypeInfo,
			Title:   "Test",
			Targets: TargetCenter,
		})
	}

	count, err := svc.GetUnreadCount(ctx)
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}

	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestMarkRead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	notif := &Notification{
		Type:    TypeInfo,
		Title:   "Test",
		Targets: TargetCenter,
	}
	svc.Send(ctx, notif)

	// Mark as read
	if err := svc.MarkRead(ctx, notif.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	// Count should be 0
	count, _ := svc.GetUnreadCount(ctx)
	if count != 0 {
		t.Errorf("count after mark read = %d, want 0", count)
	}
}

func TestMarkAllRead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Send 5 notifications
	for i := 0; i < 5; i++ {
		svc.Send(ctx, &Notification{
			Type:    TypeInfo,
			Title:   "Test",
			Targets: TargetCenter,
		})
	}

	// Mark all read
	if err := svc.MarkAllRead(ctx); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}

	count, _ := svc.GetUnreadCount(ctx)
	if count != 0 {
		t.Errorf("count after mark all = %d, want 0", count)
	}
}

func TestClearAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Send notifications
	for i := 0; i < 3; i++ {
		svc.Send(ctx, &Notification{
			Type:    TypeInfo,
			Title:   "Test",
			Targets: TargetCenter,
		})
	}

	// Clear all
	if err := svc.ClearAll(ctx); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}

	recent, _ := svc.GetRecent(ctx, 10)
	if len(recent) != 0 {
		t.Errorf("expected empty after clear, got %d", len(recent))
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Subscribe
	ch := svc.Subscribe("session1")

	// Send notification (targets toast for broadcast)
	notif := &Notification{
		Type:    TypeSuccess,
		Title:   "Broadcast test",
		Targets: TargetToast,
	}
	svc.Send(ctx, notif)

	// Should receive on channel
	select {
	case received := <-ch:
		if received.Title != "Broadcast test" {
			t.Errorf("title = %q", received.Title)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive broadcast")
	}

	// Unsubscribe
	svc.Unsubscribe("session1")

	// Channel should be closed
	_, open := <-ch
	if open {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestNotificationTypes(t *testing.T) {
	if TypeSuccess != "success" {
		t.Errorf("TypeSuccess = %q", TypeSuccess)
	}
	if TypeInfo != "info" {
		t.Errorf("TypeInfo = %q", TypeInfo)
	}
	if TypeWarning != "warning" {
		t.Errorf("TypeWarning = %q", TypeWarning)
	}
	if TypeError != "error" {
		t.Errorf("TypeError = %q", TypeError)
	}
	if TypeSecurity != "security" {
		t.Errorf("TypeSecurity = %q", TypeSecurity)
	}
}

func TestNotificationTargets(t *testing.T) {
	if TargetToast != 1 {
		t.Errorf("TargetToast = %d", TargetToast)
	}
	if TargetBanner != 2 {
		t.Errorf("TargetBanner = %d", TargetBanner)
	}
	if TargetCenter != 4 {
		t.Errorf("TargetCenter = %d", TargetCenter)
	}

	// Test bitmask combination
	combined := TargetToast | TargetCenter
	if combined&TargetToast == 0 {
		t.Error("combined should include toast")
	}
	if combined&TargetCenter == 0 {
		t.Error("combined should include center")
	}
	if combined&TargetBanner != 0 {
		t.Error("combined should not include banner")
	}
}

func TestToJSON(t *testing.T) {
	now := time.Now()
	readAt := now.Add(time.Hour)

	n := &Notification{
		ID:        "test-123",
		Type:      TypeInfo,
		Title:     "Test",
		Message:   "Message",
		CreatedAt: now,
		ReadAt:    &readAt,
		Details:   map[string]any{"key": "value"},
	}

	j := n.ToJSON()
	if j.ID != "test-123" {
		t.Errorf("ID = %q", j.ID)
	}
	if j.Unread {
		t.Error("should not be unread when ReadAt is set")
	}
	if j.ReadAt == nil {
		t.Error("ReadAt should be set")
	}
}

func TestToJSON_Unread(t *testing.T) {
	n := &Notification{
		ID:        "test-456",
		Type:      TypeWarning,
		Title:     "Unread",
		CreatedAt: time.Now(),
	}

	j := n.ToJSON()
	if !j.Unread {
		t.Error("should be unread when ReadAt is nil")
	}
}

func TestHelperFunctions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Test various helper functions
	tests := []struct {
		name string
		fn   func() error
	}{
		{"NotifySettingsSaved", func() error { return svc.NotifySettingsSaved(ctx) }},
		{"NotifyBackupComplete", func() error { return svc.NotifyBackupComplete(ctx, "backup.tar.gz", "1.2MB") }},
		{"NotifyBackupFailed", func() error { return svc.NotifyBackupFailed(ctx, "disk full") }},
		{"NotifySSLExpiring", func() error { return svc.NotifySSLExpiring(ctx, "example.com", 5) }},
		{"NotifySSLRenewed", func() error { return svc.NotifySSLRenewed(ctx, "example.com") }},
		{"NotifySchedulerTaskFailed", func() error { return svc.NotifySchedulerTaskFailed(ctx, "backup", "timeout") }},
		{"NotifyUpdateAvailable", func() error { return svc.NotifyUpdateAvailable(ctx, "1.2.0") }},
		{"NotifyRateLimitExceeded", func() error { return svc.NotifyRateLimitExceeded(ctx, "192.168.1.1") }},
		{"NotifyIPBlocked", func() error { return svc.NotifyIPBlocked(ctx, "10.0.0.1", "abuse") }},
		{"NotifySMTPNotConfigured", func() error { return svc.NotifySMTPNotConfigured(ctx) }},
		{"NotifyTorAddressReady", func() error { return svc.NotifyTorAddressReady(ctx, "abc123.onion") }},
		{"NotifyDiskSpaceLow", func() error { return svc.NotifyDiskSpaceLow(ctx, "/var/lib", 95) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err != nil {
				t.Errorf("%s: %v", tt.name, err)
			}
		})
	}
}

func TestGetRecent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := NewService(db)
	ctx := context.Background()
	svc.EnsureSchema(ctx)

	// Send some and mark some read
	n1 := &Notification{Type: TypeInfo, Title: "One", Targets: TargetCenter}
	n2 := &Notification{Type: TypeInfo, Title: "Two", Targets: TargetCenter}
	svc.Send(ctx, n1)
	svc.Send(ctx, n2)
	svc.MarkRead(ctx, n1.ID)

	// GetRecent should return both
	recent, err := svc.GetRecent(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecent: %v", err)
	}

	if len(recent) != 2 {
		t.Errorf("expected 2 recent, got %d", len(recent))
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("generateID returned empty")
	}
	if id1 == id2 {
		t.Error("generateID should return unique IDs")
	}
	if len(id1) < 10 {
		t.Errorf("ID too short: %q", id1)
	}
}

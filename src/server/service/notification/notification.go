// SPDX-License-Identifier: MIT
// AI.md PART 17: WebUI Notification System
// Toast, banner, and notification center per spec

package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NotificationType represents the type of notification per AI.md PART 17
type NotificationType string

const (
	// TypeSuccess - completed actions, confirmations (auto-dismiss 5s)
	TypeSuccess NotificationType = "success"
	// TypeInfo - informational, status updates (auto-dismiss 5s)
	TypeInfo NotificationType = "info"
	// TypeWarning - non-critical issues, expiring items (auto-dismiss 10s)
	TypeWarning NotificationType = "warning"
	// TypeError - failures, critical issues (manual dismiss)
	TypeError NotificationType = "error"
	// TypeSecurity - security-related alerts (manual dismiss)
	TypeSecurity NotificationType = "security"
)

// NotificationTarget indicates where the notification should appear
type NotificationTarget int

const (
	// TargetToast - pop-up in corner of screen
	TargetToast NotificationTarget = 1 << iota
	// TargetBanner - persistent bar at top of page
	TargetBanner
	// TargetCenter - notification center (bell icon with history)
	TargetCenter
)

// Notification represents a notification entry per AI.md PART 17
type Notification struct {
	ID        string             `json:"id"`
	Type      NotificationType   `json:"type"`
	Title     string             `json:"title"`
	Message   string             `json:"message"`
	Targets   NotificationTarget `json:"-"`
	CreatedAt time.Time          `json:"created_at"`
	ReadAt    *time.Time         `json:"read_at,omitempty"`
	Details   map[string]any     `json:"details,omitempty"`
}

// NotificationJSON is the JSON serialization format
type NotificationJSON struct {
	ID        string           `json:"id"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	CreatedAt string           `json:"created_at"`
	ReadAt    *string          `json:"read_at,omitempty"`
	Details   map[string]any   `json:"details,omitempty"`
	Unread    bool             `json:"unread"`
}

// ToJSON converts to JSON-friendly format
func (n *Notification) ToJSON() NotificationJSON {
	nj := NotificationJSON{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Message:   n.Message,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		Details:   n.Details,
		Unread:    n.ReadAt == nil,
	}
	if n.ReadAt != nil {
		s := n.ReadAt.Format(time.RFC3339)
		nj.ReadAt = &s
	}
	return nj
}

// Service manages notifications per AI.md PART 17
type Service struct {
	db *sql.DB

	// In-memory subscribers for real-time toast/banner
	subscribers map[string]chan *Notification
	subMu       sync.RWMutex
}

// NewService creates a notification service
func NewService(db *sql.DB) *Service {
	return &Service{
		db:          db,
		subscribers: make(map[string]chan *Notification),
	}
}

// EnsureSchema creates the notifications table if needed
func (s *Service) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT,
			targets INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			read_at DATETIME,
			details TEXT
		)
	`)
	return err
}

// Send creates and stores a notification
// Per AI.md PART 17: WebUI notifications always available regardless of SMTP
func (s *Service) Send(ctx context.Context, notif *Notification) error {
	if notif.ID == "" {
		notif.ID = generateID()
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}

	// Store in database if targeting notification center
	if notif.Targets&TargetCenter != 0 {
		details, _ := json.Marshal(notif.Details)
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO notifications (id, type, title, message, targets, created_at, details)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			notif.ID, string(notif.Type), notif.Title, notif.Message,
			int(notif.Targets), notif.CreatedAt, string(details))
		if err != nil {
			return fmt.Errorf("store notification: %w", err)
		}
	}

	// Broadcast to real-time subscribers
	s.broadcast(notif)

	return nil
}

// GetUnread returns all unread notifications for the notification center
func (s *Service) GetUnread(ctx context.Context, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, title, message, targets, created_at, read_at, details
		FROM notifications
		WHERE read_at IS NULL AND (targets & ?) != 0
		ORDER BY created_at DESC
		LIMIT ?`, int(TargetCenter), limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

// GetRecent returns recent notifications (read or unread)
func (s *Service) GetRecent(ctx context.Context, limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, title, message, targets, created_at, read_at, details
		FROM notifications
		WHERE (targets & ?) != 0
		ORDER BY created_at DESC
		LIMIT ?`, int(TargetCenter), limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

// GetUnreadCount returns the count of unread notifications
func (s *Service) GetUnreadCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notifications
		WHERE read_at IS NULL AND (targets & ?) != 0`,
		int(TargetCenter)).Scan(&count)
	return count, err
}

// MarkRead marks a notification as read
func (s *Service) MarkRead(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET read_at = ? WHERE id = ?`,
		time.Now(), id)
	return err
}

// MarkAllRead marks all notifications as read
func (s *Service) MarkAllRead(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET read_at = ? WHERE read_at IS NULL`,
		time.Now())
	return err
}

// ClearAll deletes all notifications
func (s *Service) ClearAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notifications`)
	return err
}

// Subscribe returns a channel for real-time notifications
// Used for SSE streaming to WebUI
func (s *Service) Subscribe(sessionID string) <-chan *Notification {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	ch := make(chan *Notification, 16)
	s.subscribers[sessionID] = ch
	return ch
}

// Unsubscribe removes a subscriber
func (s *Service) Unsubscribe(sessionID string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if ch, ok := s.subscribers[sessionID]; ok {
		close(ch)
		delete(s.subscribers, sessionID)
	}
}

// broadcast sends notification to all subscribers
func (s *Service) broadcast(notif *Notification) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- notif:
		default:
			// Channel full, skip
		}
	}
}

// scanNotifications scans notification rows
func scanNotifications(rows *sql.Rows) ([]Notification, error) {
	var notifications []Notification
	for rows.Next() {
		var n Notification
		var detailsStr sql.NullString
		var readAt sql.NullTime
		var targets int

		err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &targets,
			&n.CreatedAt, &readAt, &detailsStr)
		if err != nil {
			return nil, err
		}

		n.Targets = NotificationTarget(targets)
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		if detailsStr.Valid && detailsStr.String != "" {
			json.Unmarshal([]byte(detailsStr.String), &n.Details)
		}

		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

// generateID creates a unique notification ID
func generateID() string {
	return fmt.Sprintf("notif_%d", time.Now().UnixNano())
}

// Helper functions for common notification patterns per AI.md PART 17

// NotifySettingsSaved sends a toast notification
func (s *Service) NotifySettingsSaved(ctx context.Context) error {
	return s.Send(ctx, &Notification{
		Type:    TypeSuccess,
		Title:   "Settings saved",
		Message: "Your settings have been saved successfully.",
		Targets: TargetToast,
	})
}

// NotifyBackupComplete sends toast + center notification
func (s *Service) NotifyBackupComplete(ctx context.Context, filename, size string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeSuccess,
		Title:   "Backup completed",
		Message: filename,
		Targets: TargetToast | TargetCenter,
		Details: map[string]any{"filename": filename, "size": size},
	})
}

// NotifyBackupFailed sends toast + center notification
func (s *Service) NotifyBackupFailed(ctx context.Context, errMsg string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeError,
		Title:   "Backup failed",
		Message: errMsg,
		Targets: TargetToast | TargetCenter,
	})
}

// NotifySSLExpiring sends banner + center (urgent) or center only
func (s *Service) NotifySSLExpiring(ctx context.Context, domain string, daysLeft int) error {
	targets := TargetCenter
	if daysLeft < 3 {
		targets |= TargetBanner
	}
	return s.Send(ctx, &Notification{
		Type:    TypeWarning,
		Title:   "SSL certificate expiring",
		Message: fmt.Sprintf("Certificate for %s expires in %d days", domain, daysLeft),
		Targets: targets,
		Details: map[string]any{"domain": domain, "days_left": daysLeft},
	})
}

// NotifySSLRenewed sends toast + center notification
func (s *Service) NotifySSLRenewed(ctx context.Context, domain string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeSuccess,
		Title:   "SSL certificate renewed",
		Message: fmt.Sprintf("Certificate for %s has been renewed", domain),
		Targets: TargetToast | TargetCenter,
	})
}

// NotifySchedulerTaskFailed sends toast + center notification
func (s *Service) NotifySchedulerTaskFailed(ctx context.Context, taskName, errMsg string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeError,
		Title:   "Scheduled task failed",
		Message: fmt.Sprintf("Task '%s' failed: %s", taskName, errMsg),
		Targets: TargetToast | TargetCenter,
		Details: map[string]any{"task_name": taskName, "error": errMsg},
	})
}

// NotifyUpdateAvailable sends banner + center notification
func (s *Service) NotifyUpdateAvailable(ctx context.Context, version string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeInfo,
		Title:   "Update available",
		Message: fmt.Sprintf("Version %s is available", version),
		Targets: TargetBanner | TargetCenter,
		Details: map[string]any{"version": version},
	})
}

// NotifyRateLimitExceeded sends center notification only
func (s *Service) NotifyRateLimitExceeded(ctx context.Context, ip string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeSecurity,
		Title:   "Rate limit exceeded",
		Message: fmt.Sprintf("IP %s exceeded rate limit", ip),
		Targets: TargetCenter,
		Details: map[string]any{"ip": ip},
	})
}

// NotifyIPBlocked sends toast + center notification
func (s *Service) NotifyIPBlocked(ctx context.Context, ip, reason string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeSecurity,
		Title:   "IP blocked",
		Message: fmt.Sprintf("IP %s has been blocked: %s", ip, reason),
		Targets: TargetToast | TargetCenter,
		Details: map[string]any{"ip": ip, "reason": reason},
	})
}

// NotifySMTPNotConfigured sends persistent banner
func (s *Service) NotifySMTPNotConfigured(ctx context.Context) error {
	return s.Send(ctx, &Notification{
		Type:    TypeWarning,
		Title:   "SMTP not configured",
		Message: "Email features require SMTP configuration",
		Targets: TargetBanner,
	})
}

// NotifyTorAddressReady sends toast notification
func (s *Service) NotifyTorAddressReady(ctx context.Context, onion string) error {
	return s.Send(ctx, &Notification{
		Type:    TypeSuccess,
		Title:   "Tor address ready",
		Message: onion,
		Targets: TargetToast,
		Details: map[string]any{"onion": onion},
	})
}

// NotifyDiskSpaceLow sends banner + center notification
func (s *Service) NotifyDiskSpaceLow(ctx context.Context, path string, percentUsed int) error {
	return s.Send(ctx, &Notification{
		Type:    TypeWarning,
		Title:   "Disk space low",
		Message: fmt.Sprintf("%s is %d%% full", path, percentUsed),
		Targets: TargetBanner | TargetCenter,
		Details: map[string]any{"path": path, "percent_used": percentUsed},
	})
}

// SPDX-License-Identifier: MIT
// AI.md PART 23: Cross-Database Sync for Mixed Mode
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SyncEventType represents the type of sync event
type SyncEventType string

const (
	SyncEventInsert SyncEventType = "INSERT"
	SyncEventUpdate SyncEventType = "UPDATE"
	SyncEventDelete SyncEventType = "DELETE"
)

// SyncEvent represents a database change event for replication
type SyncEvent struct {
	ID        string        `json:"id"`
	Type      SyncEventType `json:"type"`
	Table     string        `json:"table"`
	PrimaryKey interface{}  `json:"primary_key"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	NodeID    string        `json:"node_id"`
	Version   int64         `json:"version"`
}

// SyncChannel defines the interface for sync event transport
type SyncChannel interface {
	// Publish sends a sync event to all subscribers
	Publish(ctx context.Context, event *SyncEvent) error
	// Subscribe listens for sync events
	Subscribe(ctx context.Context, handler func(*SyncEvent)) error
	// Close closes the channel
	Close() error
}

// SyncManager manages cross-database synchronization
type SyncManager struct {
	db        *AppDatabase
	channel   SyncChannel
	nodeID    string
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	enabled   bool
	version   int64
	// Tables to sync
	tables map[string]bool
	// Events pending sync
	pending   []*SyncEvent
	pendingMu sync.Mutex
}

// NewSyncManager creates a new sync manager
func NewSyncManager(db *AppDatabase, channel SyncChannel, nodeID string) *SyncManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncManager{
		db:      db,
		channel: channel,
		nodeID:  nodeID,
		ctx:     ctx,
		cancel:  cancel,
		tables:  make(map[string]bool),
		pending: make([]*SyncEvent, 0),
	}
}

// RegisterTable registers a table for synchronization
func (sm *SyncManager) RegisterTable(tableName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.tables[tableName] = true
}

// UnregisterTable unregisters a table from synchronization
func (sm *SyncManager) UnregisterTable(tableName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.tables, tableName)
}

// Start starts the sync manager
func (sm *SyncManager) Start() error {
	sm.mu.Lock()
	sm.enabled = true
	sm.mu.Unlock()

	// Start subscriber
	go func() {
		if err := sm.channel.Subscribe(sm.ctx, sm.handleEvent); err != nil {
			// Log error but don't stop
			fmt.Printf("Sync subscription error: %v\n", err)
		}
	}()

	// Start pending events processor
	go sm.processPendingEvents()

	return nil
}

// Stop stops the sync manager
func (sm *SyncManager) Stop() error {
	sm.mu.Lock()
	sm.enabled = false
	sm.mu.Unlock()

	sm.cancel()
	return sm.channel.Close()
}

// RecordChange records a database change for synchronization
func (sm *SyncManager) RecordChange(eventType SyncEventType, table string, primaryKey interface{}, data map[string]interface{}) error {
	sm.mu.RLock()
	if !sm.enabled {
		sm.mu.RUnlock()
		return nil
	}

	if !sm.tables[table] {
		sm.mu.RUnlock()
		// Table not registered for sync
		return nil
	}
	sm.mu.RUnlock()

	// Increment version
	sm.mu.Lock()
	sm.version++
	version := sm.version
	sm.mu.Unlock()

	event := &SyncEvent{
		ID:         fmt.Sprintf("%s-%d-%d", sm.nodeID, time.Now().UnixNano(), version),
		Type:       eventType,
		Table:      table,
		PrimaryKey: primaryKey,
		Data:       data,
		Timestamp:  time.Now(),
		NodeID:     sm.nodeID,
		Version:    version,
	}

	// Try to publish immediately
	if err := sm.channel.Publish(sm.ctx, event); err != nil {
		// Queue for retry
		sm.pendingMu.Lock()
		sm.pending = append(sm.pending, event)
		sm.pendingMu.Unlock()
	}

	return nil
}

// handleEvent processes incoming sync events
func (sm *SyncManager) handleEvent(event *SyncEvent) {
	// Skip our own events
	if event.NodeID == sm.nodeID {
		return
	}

	sm.mu.RLock()
	if !sm.enabled || !sm.tables[event.Table] {
		sm.mu.RUnlock()
		return
	}
	sm.mu.RUnlock()

	// Apply the change to local database
	if err := sm.applyEvent(event); err != nil {
		fmt.Printf("Failed to apply sync event: %v\n", err)
	}
}

// applyEvent applies a sync event to the local database
func (sm *SyncManager) applyEvent(event *SyncEvent) error {
	switch event.Type {
	case SyncEventInsert:
		return sm.applyInsert(event)
	case SyncEventUpdate:
		return sm.applyUpdate(event)
	case SyncEventDelete:
		return sm.applyDelete(event)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// applyInsert applies an INSERT event
func (sm *SyncManager) applyInsert(event *SyncEvent) error {
	if len(event.Data) == 0 {
		return nil
	}

	columns := make([]string, 0, len(event.Data))
	placeholders := make([]string, 0, len(event.Data))
	values := make([]interface{}, 0, len(event.Data))

	i := 1
	for col, val := range event.Data {
		columns = append(columns, col)
		if sm.db.Driver() == DriverPostgres {
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		} else {
			placeholders = append(placeholders, "?")
		}
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
		event.Table,
		joinStrings(columns, ", "),
		joinStrings(placeholders, ", "))

	_, err := sm.db.Exec(query, values...)
	return err
}

// applyUpdate applies an UPDATE event
func (sm *SyncManager) applyUpdate(event *SyncEvent) error {
	if len(event.Data) == 0 {
		return nil
	}

	sets := make([]string, 0, len(event.Data))
	values := make([]interface{}, 0, len(event.Data)+1)

	i := 1
	for col, val := range event.Data {
		if sm.db.Driver() == DriverPostgres {
			sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		} else {
			sets = append(sets, fmt.Sprintf("%s = ?", col))
		}
		values = append(values, val)
		i++
	}

	values = append(values, event.PrimaryKey)

	var query string
	if sm.db.Driver() == DriverPostgres {
		query = fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
			event.Table, joinStrings(sets, ", "), i)
	} else {
		query = fmt.Sprintf("UPDATE %s SET %s WHERE id = ?",
			event.Table, joinStrings(sets, ", "))
	}

	_, err := sm.db.Exec(query, values...)
	return err
}

// applyDelete applies a DELETE event
func (sm *SyncManager) applyDelete(event *SyncEvent) error {
	var query string
	if sm.db.Driver() == DriverPostgres {
		query = fmt.Sprintf("DELETE FROM %s WHERE id = $1", event.Table)
	} else {
		query = fmt.Sprintf("DELETE FROM %s WHERE id = ?", event.Table)
	}

	_, err := sm.db.Exec(query, event.PrimaryKey)
	return err
}

// processPendingEvents retries sending pending events
func (sm *SyncManager) processPendingEvents() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.pendingMu.Lock()
			if len(sm.pending) == 0 {
				sm.pendingMu.Unlock()
				continue
			}

			// Try to send pending events
			remaining := make([]*SyncEvent, 0)
			for _, event := range sm.pending {
				if err := sm.channel.Publish(sm.ctx, event); err != nil {
					remaining = append(remaining, event)
				}
			}
			sm.pending = remaining
			sm.pendingMu.Unlock()
		}
	}
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// MemorySyncChannel is an in-memory sync channel for single-node testing
type MemorySyncChannel struct {
	subscribers []func(*SyncEvent)
	mu          sync.RWMutex
}

// NewMemorySyncChannel creates a new in-memory sync channel
func NewMemorySyncChannel() *MemorySyncChannel {
	return &MemorySyncChannel{
		subscribers: make([]func(*SyncEvent), 0),
	}
}

// Publish sends an event to all subscribers
func (m *MemorySyncChannel) Publish(ctx context.Context, event *SyncEvent) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, handler := range m.subscribers {
		go handler(event)
	}
	return nil
}

// Subscribe adds a handler for sync events
func (m *MemorySyncChannel) Subscribe(ctx context.Context, handler func(*SyncEvent)) error {
	m.mu.Lock()
	m.subscribers = append(m.subscribers, handler)
	m.mu.Unlock()

	<-ctx.Done()
	return ctx.Err()
}

// Close closes the channel
func (m *MemorySyncChannel) Close() error {
	m.mu.Lock()
	m.subscribers = nil
	m.mu.Unlock()
	return nil
}

// ValkeySyncChannel implements SyncChannel using Valkey/Redis
type ValkeySyncChannel struct {
	client  *redis.Client
	channel string
	mu      sync.RWMutex
	closed  bool
}

// NewValkeySyncChannel creates a new Valkey/Redis sync channel
func NewValkeySyncChannel(addr, password string, db int, channel string) (*ValkeySyncChannel, error) {
	if addr == "" {
		addr = "localhost:6379"
	}
	if channel == "" {
		channel = "vidveil:sync"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &ValkeySyncChannel{
		client:  client,
		channel: channel,
	}, nil
}

// Publish sends an event via Valkey/Redis pub/sub
func (v *ValkeySyncChannel) Publish(ctx context.Context, event *SyncEvent) error {
	v.mu.RLock()
	if v.closed {
		v.mu.RUnlock()
		return fmt.Errorf("channel closed")
	}
	v.mu.RUnlock()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return v.client.Publish(ctx, v.channel, data).Err()
}

// Subscribe listens for events via Valkey/Redis pub/sub
func (v *ValkeySyncChannel) Subscribe(ctx context.Context, handler func(*SyncEvent)) error {
	v.mu.RLock()
	if v.closed {
		v.mu.RUnlock()
		return fmt.Errorf("channel closed")
	}
	v.mu.RUnlock()

	pubsub := v.client.Subscribe(ctx, v.channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-ch:
			if msg == nil {
				continue
			}
			var event SyncEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}
			handler(&event)
		}
	}
}

// Close closes the Valkey/Redis connection
func (v *ValkeySyncChannel) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.closed {
		return nil
	}
	v.closed = true

	if v.client != nil {
		return v.client.Close()
	}
	return nil
}

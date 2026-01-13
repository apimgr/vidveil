// SPDX-License-Identifier: MIT
// AI.md PART 10: Database & Cluster
package cluster

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

// Node represents a cluster node
type Node struct {
	ID            string    `json:"id"`
	Hostname      string    `json:"hostname"`
	Address       string    `json:"address"`
	Port          int       `json:"port"`
	IsPrimary     bool      `json:"is_primary"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	JoinedAt      time.Time `json:"joined_at"`
	// Status: active, inactive, or failed
	Status string `json:"status"`
}

// Lock represents a distributed lock
type Lock struct {
	Name       string    `json:"name"`
	HolderID   string    `json:"holder_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Metadata   string    `json:"metadata,omitempty"`
}

// Node state constants per AI.md PART 10
const (
	NodeStateHealthy  = "healthy"  // Heartbeat received within 30 seconds
	NodeStateDegraded = "degraded" // Heartbeat missed (30-90 seconds)
	NodeStateOffline  = "offline"  // No heartbeat for 5+ minutes
	NodeStateRemoved  = "removed"  // Manually removed by admin
)

// Timing constants per AI.md PART 10
const (
	HeartbeatInterval   = 30 * time.Second // How often nodes send heartbeats
	DegradedThreshold   = 90 * time.Second // 3 missed heartbeats = degraded
	OfflineThreshold    = 5 * time.Minute  // No heartbeat for 5 min = offline
)

// Manager handles cluster operations per AI.md PART 10
type Manager struct {
	nodeID        string
	db            *sql.DB
	isPrimary     bool
	enabled       bool
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	heartbeatInt  time.Duration
	degradedTime  time.Duration
	offlineTime   time.Duration
}

// NewManager creates a new cluster manager
func NewManager(db *sql.DB) (*Manager, error) {
	nodeID, err := generateNodeID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}

	return &Manager{
		nodeID:       nodeID,
		db:           db,
		heartbeatInt: HeartbeatInterval, // 30 seconds per PART 10
		degradedTime: DegradedThreshold, // 90 seconds per PART 10
		offlineTime:  OfflineThreshold,  // 5 minutes per PART 10
	}, nil
}

// generateNodeID generates a unique node ID
func generateNodeID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%s", hostname, hex.EncodeToString(b)), nil
}

// Start starts the cluster manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.enabled = true
	m.mu.Unlock()

	// Register this node
	if err := m.registerNode(); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start heartbeat
	go m.heartbeatLoop()

	// Start primary election
	go m.primaryElectionLoop()

	// Start lock cleanup
	go m.lockCleanupLoop()

	return nil
}

// Stop stops the cluster manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}
	m.enabled = false

	// Mark node as offline per AI.md PART 10
	if m.db != nil {
		m.db.Exec("UPDATE cluster_nodes SET status = ? WHERE id = ?", NodeStateOffline, m.nodeID)
	}
}

// registerNode registers this node in the cluster
func (m *Manager) registerNode() error {
	hostname, _ := os.Hostname()
	// Will be updated from config
	address := "0.0.0.0"
	// Will be updated from config
	port := 0

	// Use 'healthy' state per AI.md PART 10
	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO cluster_nodes (id, hostname, address, port, last_heartbeat, status)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
	`, m.nodeID, hostname, address, port, NodeStateHealthy)

	return err
}

// heartbeatLoop sends periodic heartbeats
func (m *Manager) heartbeatLoop() {
	ticker := time.NewTicker(m.heartbeatInt)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.db.Exec("UPDATE cluster_nodes SET last_heartbeat = CURRENT_TIMESTAMP WHERE id = ?", m.nodeID)
		}
	}
}

// primaryElectionLoop handles primary election per AI.md
func (m *Manager) primaryElectionLoop() {
	ticker := time.NewTicker(m.heartbeatInt * 2)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.electPrimary()
		}
	}
}

// electPrimary performs primary election per AI.md PART 10
func (m *Manager) electPrimary() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Update node states per AI.md PART 10 thresholds:
	// - healthy: heartbeat within 30 seconds
	// - degraded: heartbeat missed (30-90 seconds)
	// - offline: no heartbeat for 5+ minutes

	// Mark nodes as offline (5+ minutes without heartbeat)
	offlineThreshold := now.Add(-m.offlineTime)
	m.db.Exec(`
		UPDATE cluster_nodes
		SET status = ?, is_primary = 0
		WHERE last_heartbeat < ? AND status != ? AND status != ?
	`, NodeStateOffline, offlineThreshold, NodeStateOffline, NodeStateRemoved)

	// Mark nodes as degraded (90 seconds - 5 minutes without heartbeat)
	degradedThreshold := now.Add(-m.degradedTime)
	m.db.Exec(`
		UPDATE cluster_nodes
		SET status = ?
		WHERE last_heartbeat < ? AND last_heartbeat >= ? AND status = ?
	`, NodeStateDegraded, degradedThreshold, offlineThreshold, NodeStateHealthy)

	// Mark nodes as healthy (heartbeat within 30 seconds)
	healthyThreshold := now.Add(-m.heartbeatInt)
	m.db.Exec(`
		UPDATE cluster_nodes
		SET status = ?
		WHERE last_heartbeat >= ? AND status != ?
	`, NodeStateHealthy, healthyThreshold, NodeStateRemoved)

	// Check if there's a current primary among healthy nodes
	var primaryID string
	err := m.db.QueryRow(`
		SELECT id FROM cluster_nodes
		WHERE is_primary = 1 AND status = ?
		LIMIT 1
	`, NodeStateHealthy).Scan(&primaryID)

	if err == sql.ErrNoRows {
		// No healthy primary - elect one (node with lowest ID per PART 10)
		var electID string
		err := m.db.QueryRow(`
			SELECT id FROM cluster_nodes
			WHERE status = ?
			ORDER BY id ASC
			LIMIT 1
		`, NodeStateHealthy).Scan(&electID)

		if err == nil && electID != "" {
			m.db.Exec("UPDATE cluster_nodes SET is_primary = 0")
			m.db.Exec("UPDATE cluster_nodes SET is_primary = 1 WHERE id = ?", electID)

			if electID == m.nodeID {
				m.isPrimary = true
				fmt.Println("This node is now the primary")
			}
		}
	} else if primaryID == m.nodeID {
		m.isPrimary = true
	} else {
		m.isPrimary = false
	}
}

// lockCleanupLoop cleans up expired locks
func (m *Manager) lockCleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.db.Exec("DELETE FROM distributed_locks WHERE expires_at < CURRENT_TIMESTAMP")
		}
	}
}

// IsPrimary returns whether this node is the primary
func (m *Manager) IsPrimary() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isPrimary
}

// IsEnabled returns whether cluster mode is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// GetNodeID returns this node's ID
func (m *Manager) GetNodeID() string {
	return m.nodeID
}

// GetNodes returns all cluster nodes
func (m *Manager) GetNodes() ([]Node, error) {
	rows, err := m.db.Query(`
		SELECT id, hostname, address, port, is_primary, last_heartbeat, joined_at, status
		FROM cluster_nodes
		ORDER BY joined_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var isPrimary int
		err := rows.Scan(&n.ID, &n.Hostname, &n.Address, &n.Port, &isPrimary, &n.LastHeartbeat, &n.JoinedAt, &n.Status)
		if err != nil {
			continue
		}
		n.IsPrimary = isPrimary == 1
		nodes = append(nodes, n)
	}

	return nodes, rows.Err()
}

// AcquireLock attempts to acquire a distributed lock
func (m *Manager) AcquireLock(name string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	// Try to insert the lock
	result, err := m.db.Exec(`
		INSERT INTO distributed_locks (name, holder_id, expires_at)
		SELECT ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM distributed_locks
			WHERE name = ? AND expires_at > CURRENT_TIMESTAMP
		)
	`, name, m.nodeID, expiresAt, name)

	if err != nil {
		return false, err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		return true, nil
	}

	// Check if we already hold the lock
	var holderID string
	err = m.db.QueryRow("SELECT holder_id FROM distributed_locks WHERE name = ?", name).Scan(&holderID)
	if err == nil && holderID == m.nodeID {
		// We hold it, refresh the TTL
		m.db.Exec("UPDATE distributed_locks SET expires_at = ? WHERE name = ? AND holder_id = ?",
			expiresAt, name, m.nodeID)
		return true, nil
	}

	return false, nil
}

// ReleaseLock releases a distributed lock
func (m *Manager) ReleaseLock(name string) error {
	_, err := m.db.Exec("DELETE FROM distributed_locks WHERE name = ? AND holder_id = ?", name, m.nodeID)
	return err
}

// GetLock returns information about a lock
func (m *Manager) GetLock(name string) (*Lock, error) {
	var lock Lock
	err := m.db.QueryRow(`
		SELECT name, holder_id, acquired_at, expires_at, COALESCE(metadata, '')
		FROM distributed_locks
		WHERE name = ?
	`, name).Scan(&lock.Name, &lock.HolderID, &lock.AcquiredAt, &lock.ExpiresAt, &lock.Metadata)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &lock, nil
}

// ListLocks returns all current locks
func (m *Manager) ListLocks() ([]Lock, error) {
	rows, err := m.db.Query(`
		SELECT name, holder_id, acquired_at, expires_at, COALESCE(metadata, '')
		FROM distributed_locks
		WHERE expires_at > CURRENT_TIMESTAMP
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []Lock
	for rows.Next() {
		var l Lock
		err := rows.Scan(&l.Name, &l.HolderID, &l.AcquiredAt, &l.ExpiresAt, &l.Metadata)
		if err != nil {
			continue
		}
		locks = append(locks, l)
	}

	return locks, rows.Err()
}

// WithLock executes a function while holding a lock
func (m *Manager) WithLock(name string, ttl time.Duration, fn func() error) error {
	acquired, err := m.AcquireLock(name, ttl)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("lock '%s' is held by another node", name)
	}

	defer m.ReleaseLock(name)
	return fn()
}

// Stats returns cluster statistics
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"node_id":    m.nodeID,
		"is_primary": m.isPrimary,
		"enabled":    m.enabled,
	}

	// Count nodes
	var totalNodes, activeNodes int
	m.db.QueryRow("SELECT COUNT(*) FROM cluster_nodes").Scan(&totalNodes)
	m.db.QueryRow("SELECT COUNT(*) FROM cluster_nodes WHERE status = ?", NodeStateHealthy).Scan(&activeNodes)

	stats["total_nodes"] = totalNodes
	stats["active_nodes"] = activeNodes

	// Count locks
	var lockCount int
	m.db.QueryRow("SELECT COUNT(*) FROM distributed_locks WHERE expires_at > CURRENT_TIMESTAMP").Scan(&lockCount)
	stats["active_locks"] = lockCount

	return stats
}

// SingleInstanceManager is a no-op cluster manager for single instance mode
type SingleInstanceManager struct {
	nodeID string
}

// NewSingleInstanceManager creates a manager for single instance mode
func NewSingleInstanceManager() *SingleInstanceManager {
	nodeID, _ := generateNodeID()
	return &SingleInstanceManager{nodeID: nodeID}
}

func (s *SingleInstanceManager) IsPrimary() bool     { return true }
func (s *SingleInstanceManager) IsEnabled() bool     { return false }
func (s *SingleInstanceManager) GetNodeID() string   { return s.nodeID }
func (s *SingleInstanceManager) Start(ctx context.Context) error { return nil }
func (s *SingleInstanceManager) Stop()               {}
func (s *SingleInstanceManager) AcquireLock(name string, ttl time.Duration) (bool, error) { return true, nil }
func (s *SingleInstanceManager) ReleaseLock(name string) error { return nil }
func (s *SingleInstanceManager) WithLock(name string, ttl time.Duration, fn func() error) error { return fn() }

// GenerateJoinToken generates a secure random join token for cluster
func GenerateJoinToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

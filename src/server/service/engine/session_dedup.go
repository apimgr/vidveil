// SPDX-License-Identifier: MIT
package engine

import (
	"sync"
	"time"
)

// sessionSeenTTL is how long a search session's seen-results state is
// retained after its last access. Infinite-scroll pagination issues one
// request per page, so this only needs to outlive a single scrolling
// session, not a whole visit.
const sessionSeenTTL = 20 * time.Minute

// maxDedupSessions bounds memory use so the store can never grow
// unbounded (see AI.md backend rules: "Cache with TTL — never unbounded
// caches").
const maxDedupSessions = 5000

// sessionSeen tracks the normalized URLs and titles already returned to
// a single search session, so subsequent pages of the same infinite-scroll
// search do not resurface the same video result.
type sessionSeen struct {
	urls       map[string]bool
	titles     []string
	lastAccess time.Time
}

// SessionDedupStore is a TTL-bounded, in-memory store of per-session
// "already returned" result state. It lets Search/SearchStreamWithOperators
// exclude results already delivered on an earlier page of the same
// client-side search session, without requiring the client to perform any
// deduplication itself.
type SessionDedupStore struct {
	mu       sync.Mutex
	sessions map[string]*sessionSeen
	ttl      time.Duration
}

// NewSessionDedupStore creates a session dedup store and starts its
// background cleanup goroutine.
func NewSessionDedupStore() *SessionDedupStore {
	s := &SessionDedupStore{
		sessions: make(map[string]*sessionSeen),
		ttl:      sessionSeenTTL,
	}
	go s.cleanupLoop()
	return s
}

// CheckAndMark reports whether normalizedURL (or a fuzzy-duplicate title)
// has already been returned for sessionID. If not, it records both as seen
// for future pages of the same session. An empty sessionID is a no-op that
// always reports "not a duplicate" — callers without a session (e.g. plain
// JSON API consumers that never paginate) keep the prior single-request
// dedup behavior.
func (s *SessionDedupStore) CheckAndMark(sessionID, normalizedURL, normalizedTitle string) bool {
	if sessionID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		if len(s.sessions) >= maxDedupSessions {
			s.evictOldestLocked()
		}
		sess = &sessionSeen{urls: make(map[string]bool)}
		s.sessions[sessionID] = sess
	}
	sess.lastAccess = time.Now()

	if sess.urls[normalizedURL] {
		return true
	}

	if normalizedTitle != "" {
		for _, seen := range sess.titles {
			if titlesAreFuzzyDuplicates(normalizedTitle, seen) {
				return true
			}
		}
	}

	sess.urls[normalizedURL] = true
	if normalizedTitle != "" {
		sess.titles = append(sess.titles, normalizedTitle)
	}
	return false
}

// evictOldestLocked removes the least-recently-accessed session. Caller
// must hold s.mu.
func (s *SessionDedupStore) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range s.sessions {
		if first || v.lastAccess.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.lastAccess
			first = false
		}
	}
	if oldestKey != "" {
		delete(s.sessions, oldestKey)
	}
}

// cleanupLoop periodically removes sessions that have been idle past the TTL.
func (s *SessionDedupStore) cleanupLoop() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.sessions {
			if now.Sub(v.lastAccess) > s.ttl {
				delete(s.sessions, k)
			}
		}
		s.mu.Unlock()
	}
}

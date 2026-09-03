// SPDX-License-Identifier: MIT
// AI.md PART 28: Unit tests for SessionDedupStore (session_dedup.go).
package engine

import (
	"testing"
	"time"
)

func TestSessionDedupStore_EmptySessionID_AlwaysNotDuplicate(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: sessionSeenTTL}

	if s.CheckAndMark("", "https://example.com/v1", "title") {
		t.Fatal("empty sessionID must never report a duplicate")
	}
	if s.CheckAndMark("", "https://example.com/v1", "title") {
		t.Fatal("empty sessionID must never report a duplicate, even on repeat calls")
	}
	if len(s.sessions) != 0 {
		t.Fatalf("empty sessionID must not create session state, got %d sessions", len(s.sessions))
	}
}

func TestSessionDedupStore_SameSession_URLDeduped(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: sessionSeenTTL}

	if s.CheckAndMark("sess1", "https://example.com/v1", "") {
		t.Fatal("first sighting of a URL must not be a duplicate")
	}
	if !s.CheckAndMark("sess1", "https://example.com/v1", "") {
		t.Fatal("second sighting of the same URL in the same session must be a duplicate")
	}
}

func TestSessionDedupStore_SameSession_FuzzyTitleDeduped(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: sessionSeenTTL}

	if s.CheckAndMark("sess1", "https://example.com/v1", "pregnant lesbian teen") {
		t.Fatal("first sighting must not be a duplicate")
	}
	// Different URL, fuzzy-duplicate title -> must still be caught.
	if !s.CheckAndMark("sess1", "https://example.com/v2", "pregnant lesbian teen") {
		t.Fatal("fuzzy-duplicate title under a different URL must be flagged as duplicate")
	}
}

func TestSessionDedupStore_DifferentSessions_NotShared(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: sessionSeenTTL}

	if s.CheckAndMark("sess1", "https://example.com/v1", "") {
		t.Fatal("first sighting in sess1 must not be a duplicate")
	}
	if s.CheckAndMark("sess2", "https://example.com/v1", "") {
		t.Fatal("same URL in a different session must not be treated as a duplicate")
	}
}

func TestSessionDedupStore_EvictOldestLocked_RemovesLeastRecentlyUsed(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: sessionSeenTTL}

	now := time.Now()
	s.sessions["old"] = &sessionSeen{urls: make(map[string]bool), lastAccess: now.Add(-time.Hour)}
	s.sessions["mid"] = &sessionSeen{urls: make(map[string]bool), lastAccess: now.Add(-time.Minute)}
	s.sessions["new"] = &sessionSeen{urls: make(map[string]bool), lastAccess: now}

	s.mu.Lock()
	s.evictOldestLocked()
	s.mu.Unlock()

	if _, ok := s.sessions["old"]; ok {
		t.Fatal("evictOldestLocked must remove the least-recently-accessed session")
	}
	if len(s.sessions) != 2 {
		t.Fatalf("expected 2 remaining sessions, got %d", len(s.sessions))
	}
}

func TestSessionDedupStore_CheckAndMark_EvictsWhenAtCapacity(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: sessionSeenTTL}

	now := time.Now()
	for i := 0; i < maxDedupSessions; i++ {
		key := time.Duration(i).String()
		s.sessions[key] = &sessionSeen{urls: make(map[string]bool), lastAccess: now.Add(time.Duration(i) * time.Millisecond)}
	}
	if len(s.sessions) != maxDedupSessions {
		t.Fatalf("setup: expected %d sessions, got %d", maxDedupSessions, len(s.sessions))
	}

	// Adding one more session while at capacity must evict an old one
	// rather than growing unbounded.
	s.CheckAndMark("brand-new-session", "https://example.com/v1", "")

	if len(s.sessions) != maxDedupSessions {
		t.Fatalf("expected session count to stay bounded at %d, got %d", maxDedupSessions, len(s.sessions))
	}
	if _, ok := s.sessions["brand-new-session"]; !ok {
		t.Fatal("newly added session must be present after eviction")
	}
}

func TestSessionDedupStore_CleanupLoop_RemovesExpiredSessions(t *testing.T) {
	s := &SessionDedupStore{sessions: make(map[string]*sessionSeen), ttl: 20 * time.Millisecond}

	s.mu.Lock()
	s.sessions["expiring"] = &sessionSeen{urls: make(map[string]bool), lastAccess: time.Now().Add(-time.Hour)}
	s.mu.Unlock()

	go s.cleanupLoop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		_, ok := s.sessions["expiring"]
		s.mu.Unlock()
		if !ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cleanupLoop did not remove an expired session within the deadline")
}

func TestNewSessionDedupStore_ReturnsUsableStore(t *testing.T) {
	s := NewSessionDedupStore()
	if s == nil {
		t.Fatal("NewSessionDedupStore returned nil")
	}
	if s.CheckAndMark("sess", "https://example.com/v1", "") {
		t.Fatal("first sighting on a fresh store must not be a duplicate")
	}
	if !s.CheckAndMark("sess", "https://example.com/v1", "") {
		t.Fatal("second sighting must be a duplicate")
	}
}

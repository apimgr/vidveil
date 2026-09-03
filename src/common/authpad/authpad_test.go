// SPDX-License-Identifier: MIT
package authpad

import (
	"testing"
	"time"
)

func TestWait_PadsToFloor(t *testing.T) {
	start := time.Now()
	Wait(start)
	elapsed := time.Since(start)
	if elapsed < Floor {
		t.Fatalf("expected elapsed >= floor %v, got %v", Floor, elapsed)
	}
}

func TestWait_DoesNotDoublePadSlowPath(t *testing.T) {
	start := time.Now().Add(-2 * Floor)
	before := time.Now()
	Wait(start)
	elapsed := time.Since(before)
	if elapsed > 20*time.Millisecond {
		t.Fatalf("expected no additional sleep for already-slow path, slept %v", elapsed)
	}
}

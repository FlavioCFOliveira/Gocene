// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.misc.util.TestCollectorMemoryTracker.
// The Java class lives in org.apache.lucene.misc.CollectorMemoryTracker;
// its Go port lives in the misc package.
package util_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/misc"
)

// TestCollectorMemoryTracker_AdditionsAndDeletions mirrors testAdditionsAndDeletions.
// Verifies that:
//   - Additions and subtractions update the counter correctly.
//   - Exceeding the limit returns an error (state is updated first, matching
//     Java's AtomicLong.addAndGet semantics).
//   - Going below zero returns an error.
func TestCollectorMemoryTracker_AdditionsAndDeletions(t *testing.T) {
	const perCollectorMemoryLimit = 100 // 100 bytes
	tracker := misc.NewCollectorMemoryTrackerNamed("testMemoryTracker", perCollectorMemoryLimit)

	// +50 → 50
	if err := tracker.UpdateBytes(50); err != nil {
		t.Fatalf("updateBytes(50): unexpected error: %v", err)
	}
	if got := tracker.GetBytes(); got != 50 {
		t.Errorf("after +50: expected 50, got %d", got)
	}

	// -30 → 20
	if err := tracker.UpdateBytes(-30); err != nil {
		t.Fatalf("updateBytes(-30): unexpected error: %v", err)
	}
	if got := tracker.GetBytes(); got != 20 {
		t.Errorf("after -30: expected 20, got %d", got)
	}

	// +130 → 150 > 100: must return error (state becomes 150)
	if err := tracker.UpdateBytes(130); err == nil {
		t.Fatal("updateBytes(130): expected error for exceeding limit, got nil")
	}

	// -110 → 150 - 110 = 40: no error
	if err := tracker.UpdateBytes(-110); err != nil {
		t.Fatalf("updateBytes(-110): unexpected error: %v", err)
	}
	if got := tracker.GetBytes(); got != 40 {
		t.Errorf("after -110: expected 40, got %d", got)
	}

	// -90 → 40 - 90 = -50 < 0: must return error
	if err := tracker.UpdateBytes(-90); err == nil {
		t.Fatal("updateBytes(-90): expected error for going below zero, got nil")
	}
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package misc

import (
	"fmt"
	"sync/atomic"
)

// CollectorMemoryTracker accounts for the bytes a collector retains and
// enforces a per-collector memory limit. Mirrors
// org.apache.lucene.misc.CollectorMemoryTracker.
//
// Key contract (matching the Java implementation): UpdateBytes applies the
// delta atomically before checking limits. If the resulting value exceeds the
// limit or drops below zero, an error is returned AND the internal state
// retains the post-delta value (matching AtomicLong semantics in Java).
type CollectorMemoryTracker struct {
	name         string
	memoryLimit  int64
	memoryUsage  atomic.Int64
}

// NewCollectorMemoryTrackerNamed creates a tracker with a name and a byte
// limit, matching the Java constructor CollectorMemoryTracker(String,long).
func NewCollectorMemoryTrackerNamed(name string, memoryLimit int64) *CollectorMemoryTracker {
	return &CollectorMemoryTracker{
		name:        name,
		memoryLimit: memoryLimit,
	}
}

// NewCollectorMemoryTracker creates an unnamed tracker for backward
// compatibility with existing callers in the Gocene tree.
func NewCollectorMemoryTracker(maxBytes int64) *CollectorMemoryTracker {
	return NewCollectorMemoryTrackerNamed("", maxBytes)
}

// UpdateBytes applies delta (positive or negative) to the current byte count.
// If the result exceeds the limit, an error is returned (the internal state
// is already updated, matching the Java AtomicLong behaviour). If the result
// drops below zero, an error is returned with the same semantics.
func (t *CollectorMemoryTracker) UpdateBytes(bytes int64) error {
	current := t.memoryUsage.Add(bytes)
	if current > t.memoryLimit {
		return fmt.Errorf("memory limit exceeded for %s", t.name)
	}
	if current < 0 {
		return fmt.Errorf("illegal memory state for %s", t.name)
	}
	return nil
}

// GetBytes returns the current tracked byte count.
func (t *CollectorMemoryTracker) GetBytes() int64 {
	return t.memoryUsage.Load()
}

// Track records bytes used; returns false when the budget would be exceeded.
// Deprecated: prefer UpdateBytes for direct compatibility with Java semantics.
func (t *CollectorMemoryTracker) Track(bytes int64) bool {
	return t.UpdateBytes(bytes) == nil
}

// Used returns the current allocation.
// Deprecated: prefer GetBytes.
func (t *CollectorMemoryTracker) Used() int64 { return t.GetBytes() }

// Release subtracts bytes from the budget.
// Deprecated: prefer UpdateBytes with a negative delta.
func (t *CollectorMemoryTracker) Release(bytes int64) {
	// Best-effort: ignore the error as the old API did not propagate errors.
	_ = t.UpdateBytes(-bytes)
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "time"

// TimeLimitingKnnCollectorManager wraps an inner KnnCollectorManager-like
// factory with a deadline, signalling early termination once the deadline
// elapses.
//
// Mirrors org.apache.lucene.search.TimeLimitingKnnCollectorManager. The
// underlying factory contract is intentionally minimal here: the manager
// returned is a TopKnnCollector with a Deadline()-aware variant.
type TimeLimitingKnnCollectorManager struct {
	k          int
	visitLimit int
	deadline   time.Time
}

// NewTimeLimitingKnnCollectorManager builds a manager that produces
// TopKnnCollectors and signals early termination if deadline has passed.
func NewTimeLimitingKnnCollectorManager(k, visitLimit int, deadline time.Time) *TimeLimitingKnnCollectorManager {
	return &TimeLimitingKnnCollectorManager{k: k, visitLimit: visitLimit, deadline: deadline}
}

// NewCollector returns a freshly-created TopKnnCollector. The deadline is
// applied lazily by callers via DeadlineReached.
func (m *TimeLimitingKnnCollectorManager) NewCollector() *TopKnnCollector {
	return NewTopKnnCollector(m.k, m.visitLimit)
}

// DeadlineReached reports whether the configured deadline has elapsed.
func (m *TimeLimitingKnnCollectorManager) DeadlineReached() bool {
	if m.deadline.IsZero() {
		return false
	}
	return time.Now().After(m.deadline)
}

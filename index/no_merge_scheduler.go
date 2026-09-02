// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// NoMergeScheduler is a singleton MergeScheduler that performs no merges.
// Mirrors org.apache.lucene.index.NoMergeScheduler from Apache Lucene 10.4.0.
type NoMergeScheduler struct{}

// NoMergeSchedulerInstance is the canonical singleton instance.
var NoMergeSchedulerInstance MergeScheduler = newNoMergeSchedulerSingleton()

func newNoMergeSchedulerSingleton() MergeScheduler {
	return &NoMergeScheduler{}
}

// Merge is a no-op.
func (n *NoMergeScheduler) Merge(_ MergeSource, _ MergeTrigger) error { return nil }

// Close is a no-op.
func (n *NoMergeScheduler) Close() error { return nil }

// GetRunningMergeCount always returns 0.
func (n *NoMergeScheduler) GetRunningMergeCount() int { return 0 }

// SetMaxMerges is a no-op.
func (n *NoMergeScheduler) SetMaxMerges(_ int) {}

// GetMaxMerges always returns 0.
func (n *NoMergeScheduler) GetMaxMerges() int { return 0 }

// Clone returns the singleton (NoMergeScheduler is stateless).
func (n *NoMergeScheduler) Clone() MergeScheduler { return n }

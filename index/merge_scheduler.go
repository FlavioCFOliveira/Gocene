// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// MergeScheduler schedules background merges.
type MergeScheduler interface {
	// Merge schedules a merge.
	Merge(writer *IndexWriter, merge *OneMerge) error

	// Close closes the scheduler.
	Close() error
}

// BaseMergeScheduler provides common functionality.
type BaseMergeScheduler struct{}

// Merge schedules a merge.
func (s *BaseMergeScheduler) Merge(writer *IndexWriter, merge *OneMerge) error {
	return nil
}

// Close closes the scheduler.
func (s *BaseMergeScheduler) Close() error {
	return nil
}

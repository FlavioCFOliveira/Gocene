// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
)

// SerialMergeScheduler is a MergeScheduler that simply does each merge sequentially, using the current thread.
// Mirrors org.apache.lucene.index.SerialMergeScheduler from Apache Lucene 10.5.0.
type SerialMergeScheduler struct {
	*BaseMergeScheduler

	mu sync.Mutex
}

// NewSerialMergeScheduler constructs a SerialMergeScheduler.
func NewSerialMergeScheduler() *SerialMergeScheduler {
	return &SerialMergeScheduler{BaseMergeScheduler: NewBaseMergeScheduler()}
}

func (s *SerialMergeScheduler) Merge(mergeSource MergeSource, trigger MergeTrigger) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		merge := mergeSource.GetNextMerge()
		if merge == nil {
			break
		}
		if err := mergeSource.Merge(merge); err != nil {
			return err
		}
	}
	return nil
}

func (s *SerialMergeScheduler) Close() error {
	return nil
}

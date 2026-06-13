// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestMultiIndexMergeScheduler exercises the multi-tenant merge scheduler.
func TestMultiIndexMergeScheduler(t *testing.T) {
	t.Run("constructor with explicit combined scheduler", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		combined := index.NewCombinedMergeScheduler()
		s := index.NewMultiIndexMergeSchedulerWith(dir, combined)

		if s.GetDirectory() != dir {
			t.Error("GetDirectory() did not return the constructor directory")
		}
		if s.GetCombinedMergeScheduler() != combined {
			t.Error("GetCombinedMergeScheduler() did not return the constructor scheduler")
		}

		// manageSingleton is false here: Close must not touch the singleton.
		if err := s.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("merge routes tagged source to combined scheduler", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		combined := index.NewCombinedMergeScheduler()
		s := index.NewMultiIndexMergeSchedulerWith(dir, combined)
		defer s.Close()

		merge := index.NewOneMerge([]*index.SegmentCommitInfo{})
		source := newMockMergeSource([]*index.OneMerge{merge})

		if err := s.Merge(source, index.EXPLICIT); err != nil {
			t.Fatalf("Merge() error = %v", err)
		}
		if source.MergeCount() != 1 {
			t.Errorf("mergeCount = %d, want 1", source.MergeCount())
		}
		if source.FinishedCount() != 1 {
			t.Errorf("len(finished) = %d, want 1", source.FinishedCount())
		}
	})

	t.Run("singleton acquire and release ref counting", func(t *testing.T) {
		if index.PeekCombinedSingleton() != nil {
			t.Fatal("singleton already allocated by another test; skipping")
		}

		d1 := store.NewByteBuffersDirectory()
		d2 := store.NewByteBuffersDirectory()
		s1 := index.NewMultiIndexMergeScheduler(d1)
		s2 := index.NewMultiIndexMergeScheduler(d2)

		if s1.GetCombinedMergeScheduler() != s2.GetCombinedMergeScheduler() {
			t.Error("two MultiIndexMergeSchedulers must share one CombinedMergeScheduler")
		}
		if index.PeekCombinedSingleton() == nil {
			t.Error("PeekCombinedSingleton() = nil while references are held")
		}

		if err := s1.Close(); err != nil {
			t.Errorf("s1.Close() error = %v", err)
		}
		if index.PeekCombinedSingleton() == nil {
			t.Error("singleton released while s2 still holds a reference")
		}
		if err := s2.Close(); err != nil {
			t.Errorf("s2.Close() error = %v", err)
		}
		if index.PeekCombinedSingleton() != nil {
			t.Error("singleton not released after last reference closed")
		}
	})
}

// TestTaggedMergeSource verifies the tagged source is a transparent pass-through.
func TestTaggedMergeSource(t *testing.T) {
	t.Run("interface compliance", func(t *testing.T) {
		var _ index.MergeSource = (*index.TaggedMergeSource)(nil)
	})

	t.Run("combined scheduler rejects an untagged source", func(t *testing.T) {
		combined := index.NewCombinedMergeScheduler()
		defer combined.Close()

		source := newMockMergeSource(nil)
		if err := combined.Merge(source, index.EXPLICIT); err == nil {
			t.Error("Merge() with an untagged source should return an error")
		}
	})
}

// TestCombinedMergeSchedulerSync verifies Sync returns once a directory is idle.
func TestCombinedMergeSchedulerSync(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	combined := index.NewCombinedMergeScheduler()
	defer combined.Close()

	s := index.NewMultiIndexMergeSchedulerWith(dir, combined)
	defer s.Close()

	merge := index.NewOneMerge([]*index.SegmentCommitInfo{})
	source := newMockMergeSource([]*index.OneMerge{merge})
	if err := s.Merge(source, index.EXPLICIT); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	// Merge has returned, so the directory must already be idle; Sync must not block.
	combined.Sync(dir)
}

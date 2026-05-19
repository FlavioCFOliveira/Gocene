// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// countingMergeContext is a MergeContext stub that counts how many times each
// method is invoked, so tests can assert the caching contract of
// CachingMergeContext.
type countingMergeContext struct {
	numDeletesCalls    map[*SegmentCommitInfo]int
	numDeletedDocsCall int
	infoStreamCalls    int
	mergingCalls       int

	deletesByInfo map[*SegmentCommitInfo]int
	deletedByInfo map[*SegmentCommitInfo]int
	infoStream    InfoStream
	merging       map[*SegmentCommitInfo]bool
}

func newCountingMergeContext() *countingMergeContext {
	return &countingMergeContext{
		numDeletesCalls: make(map[*SegmentCommitInfo]int),
		deletesByInfo:   make(map[*SegmentCommitInfo]int),
		deletedByInfo:   make(map[*SegmentCommitInfo]int),
		merging:         make(map[*SegmentCommitInfo]bool),
	}
}

func (c *countingMergeContext) NumDeletesToMerge(info *SegmentCommitInfo) int {
	c.numDeletesCalls[info]++
	return c.deletesByInfo[info]
}

func (c *countingMergeContext) NumDeletedDocs(info *SegmentCommitInfo) int {
	c.numDeletedDocsCall++
	return c.deletedByInfo[info]
}

func (c *countingMergeContext) GetInfoStream() InfoStream {
	c.infoStreamCalls++
	return c.infoStream
}

func (c *countingMergeContext) GetMergingSegments() map[*SegmentCommitInfo]bool {
	c.mergingCalls++
	return c.merging
}

func TestCachingMergeContext_NumDeletesToMergeIsCachedPerInfo(t *testing.T) {
	t.Parallel()

	a := &SegmentCommitInfo{}
	b := &SegmentCommitInfo{}

	delegate := newCountingMergeContext()
	delegate.deletesByInfo[a] = 3
	delegate.deletesByInfo[b] = 7

	c := NewCachingMergeContext(delegate)

	for i := 0; i < 5; i++ {
		if got := c.NumDeletesToMerge(a); got != 3 {
			t.Fatalf("a iter %d: got %d, want 3", i, got)
		}
		if got := c.NumDeletesToMerge(b); got != 7 {
			t.Fatalf("b iter %d: got %d, want 7", i, got)
		}
	}

	if delegate.numDeletesCalls[a] != 1 {
		t.Fatalf("delegate NumDeletesToMerge(a) calls = %d, want 1", delegate.numDeletesCalls[a])
	}
	if delegate.numDeletesCalls[b] != 1 {
		t.Fatalf("delegate NumDeletesToMerge(b) calls = %d, want 1", delegate.numDeletesCalls[b])
	}
}

func TestCachingMergeContext_NonCachedMethodsAlwaysDelegate(t *testing.T) {
	t.Parallel()

	info := &SegmentCommitInfo{}
	delegate := newCountingMergeContext()
	delegate.deletedByInfo[info] = 11
	delegate.merging[info] = true

	c := NewCachingMergeContext(delegate)

	for i := 0; i < 3; i++ {
		if got := c.NumDeletedDocs(info); got != 11 {
			t.Fatalf("NumDeletedDocs iter %d: got %d, want 11", i, got)
		}
		_ = c.GetInfoStream()
		ms := c.GetMergingSegments()
		if !ms[info] {
			t.Fatalf("GetMergingSegments missing info on iter %d", i)
		}
	}

	if delegate.numDeletedDocsCall != 3 {
		t.Fatalf("NumDeletedDocs delegate calls = %d, want 3", delegate.numDeletedDocsCall)
	}
	if delegate.infoStreamCalls != 3 {
		t.Fatalf("GetInfoStream delegate calls = %d, want 3", delegate.infoStreamCalls)
	}
	if delegate.mergingCalls != 3 {
		t.Fatalf("GetMergingSegments delegate calls = %d, want 3", delegate.mergingCalls)
	}
}

func TestCachingMergeContext_ImplementsMergeContext(t *testing.T) {
	t.Parallel()

	var _ MergeContext = (*CachingMergeContext)(nil)
}

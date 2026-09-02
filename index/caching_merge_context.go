// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// CachingMergeContext is a wrapper of MergeContext that caches the result of
// NumDeletesToMerge during the merge phase, to avoid duplicate calculation.
//
// This is the Go port of org.apache.lucene.index.CachingMergeContext.
//
// Unlike the JVM original, NumDeletesToMerge does not return an error: the
// Gocene MergeContext interface (see merge_context.go) returns int only.
// The semantics are otherwise identical: first lookup is delegated, subsequent
// lookups for the same SegmentCommitInfo are served from the cache.
//
// CachingMergeContext is not safe for concurrent use. The reference Java
// implementation uses a plain HashMap (no synchronization) and is expected to
// be used by a single merge-selection goroutine at a time.
type CachingMergeContext struct {
	mergeContext            MergeContext
	cachedNumDeletesToMerge map[*SegmentCommitInfo]int
}

// NewCachingMergeContext wraps the given MergeContext with a per-segment cache
// of NumDeletesToMerge results.
func NewCachingMergeContext(mergeContext MergeContext) *CachingMergeContext {
	return &CachingMergeContext{
		mergeContext:            mergeContext,
		cachedNumDeletesToMerge: make(map[*SegmentCommitInfo]int),
	}
}

// NumDeletesToMerge returns the number of deletes a merge would claim back
// for the given segment, caching the delegate's answer on first call.
func (c *CachingMergeContext) NumDeletesToMerge(info *SegmentCommitInfo) int {
	if n, ok := c.cachedNumDeletesToMerge[info]; ok {
		return n
	}
	n := c.mergeContext.NumDeletesToMerge(info)
	c.cachedNumDeletesToMerge[info] = n
	return n
}

// NumDeletedDocs delegates to the wrapped MergeContext without caching.
func (c *CachingMergeContext) NumDeletedDocs(info *SegmentCommitInfo) int {
	return c.mergeContext.NumDeletedDocs(info)
}

// GetInfoStream delegates to the wrapped MergeContext.
func (c *CachingMergeContext) GetInfoStream() InfoStream {
	return c.mergeContext.GetInfoStream()
}

// GetMergingSegments delegates to the wrapped MergeContext.
func (c *CachingMergeContext) GetMergingSegments() map[*SegmentCommitInfo]bool {
	return c.mergeContext.GetMergingSegments()
}

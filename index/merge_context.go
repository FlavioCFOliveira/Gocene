// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// MergeContext provides context for merge selection.
// This is the Go port of Lucene's org.apache.lucene.index.MergePolicy.MergeContext.
//
// MergeContext represents the current context of the merge selection process.
// It allows access to real-time information like the currently merging segments
// or how many deletes a segment would claim back if merged.
type MergeContext interface {
	// NumDeletesToMerge returns the number of deleted documents a merge would
	// claim back if the given segment is merged.
	// This includes both on-disk deletes and pending deletes.
	NumDeletesToMerge(info *SegmentCommitInfo) int

	// NumDeletedDocs returns the number of deleted documents in the segment.
	// This is the count of deleted docs stored in the segment's live docs.
	NumDeletedDocs(info *SegmentCommitInfo) int

	// GetInfoStream returns the info stream for logging merge messages.
	// Returns nil if no info stream is configured.
	GetInfoStream() InfoStream

	// GetMergingSegments returns the set of segments that are currently being merged.
	// This is used to avoid selecting segments that are already being merged.
	GetMergingSegments() map[*SegmentCommitInfo]bool
}

// BaseMergeContext provides a basic implementation of MergeContext.
type BaseMergeContext struct {
	// numDeletesToMerge maps segment info to delete counts
	numDeletesToMergeMap map[*SegmentCommitInfo]int

	// numDeletedDocs maps segment info to deleted docs count
	numDeletedDocsMap map[*SegmentCommitInfo]int

	// mergingSegments is the set of currently merging segments
	mergingSegments map[*SegmentCommitInfo]bool

	// infoStream is the optional info stream for logging
	infoStream InfoStream
}

// NewBaseMergeContext creates a new BaseMergeContext.
func NewBaseMergeContext() *BaseMergeContext {
	return &BaseMergeContext{
		numDeletesToMergeMap: make(map[*SegmentCommitInfo]int),
		numDeletedDocsMap:    make(map[*SegmentCommitInfo]int),
		mergingSegments:      make(map[*SegmentCommitInfo]bool),
	}
}

// NumDeletesToMerge returns the number of deletes to merge for the segment.
func (c *BaseMergeContext) NumDeletesToMerge(info *SegmentCommitInfo) int {
	if count, ok := c.numDeletesToMergeMap[info]; ok {
		return count
	}
	// Default: return the del count from the segment
	return info.DelCount()
}

// NumDeletedDocs returns the number of deleted docs in the segment.
func (c *BaseMergeContext) NumDeletedDocs(info *SegmentCommitInfo) int {
	if count, ok := c.numDeletedDocsMap[info]; ok {
		return count
	}
	return info.DelCount()
}

// GetInfoStream returns the info stream for logging.
func (c *BaseMergeContext) GetInfoStream() InfoStream {
	return c.infoStream
}

// GetMergingSegments returns the set of currently merging segments.
func (c *BaseMergeContext) GetMergingSegments() map[*SegmentCommitInfo]bool {
	return c.mergingSegments
}

// SetNumDeletesToMerge sets the number of deletes to merge for a segment.
func (c *BaseMergeContext) SetNumDeletesToMerge(info *SegmentCommitInfo, count int) {
	c.numDeletesToMergeMap[info] = count
}

// SetNumDeletedDocs sets the number of deleted docs for a segment.
func (c *BaseMergeContext) SetNumDeletedDocs(info *SegmentCommitInfo, count int) {
	c.numDeletedDocsMap[info] = count
}

// SetInfoStream sets the info stream for logging.
func (c *BaseMergeContext) SetInfoStream(infoStream InfoStream) {
	c.infoStream = infoStream
}

// AddMergingSegment adds a segment to the set of currently merging segments.
func (c *BaseMergeContext) AddMergingSegment(info *SegmentCommitInfo) {
	c.mergingSegments[info] = true
}

// RemoveMergingSegment removes a segment from the set of currently merging segments.
func (c *BaseMergeContext) RemoveMergingSegment(info *SegmentCommitInfo) {
	delete(c.mergingSegments, info)
}

// SegmentSizeAndDocs holds segment size information for merge selection.
// This is the Go port of Lucene's TieredMergePolicy.SegmentSizeAndDocs.
type SegmentSizeAndDocs struct {
	// SegInfo is the segment this wraps
	SegInfo *SegmentCommitInfo

	// SizeInBytes is the size of all segment files in bytes (pro-rated by deletes)
	SizeInBytes int64

	// DelCount is the number of deleted documents in this segment
	DelCount int

	// MaxDoc is the maximum document ID in this segment (total docs)
	MaxDoc int

	// Name is the name of the segment
	Name string
}

// NewSegmentSizeAndDocs creates a new SegmentSizeAndDocs from segment info.
func NewSegmentSizeAndDocs(info *SegmentCommitInfo, sizeInBytes int64, delCount int) *SegmentSizeAndDocs {
	return &SegmentSizeAndDocs{
		SegInfo:     info,
		SizeInBytes: sizeInBytes,
		DelCount:    delCount,
		MaxDoc:      info.SegmentInfo().DocCount(),
		Name:        info.Name(),
	}
}

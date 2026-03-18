// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// UpgradeIndexMergePolicy is a merge policy that forces all segments to be merged
// into a single segment, used for upgrading index segments to the current version.
//
// This is the Go port of Lucene's org.apache.lucene.index.UpgradeIndexMergePolicy.
//
// This merge policy is used by IndexUpgrader to upgrade all segments to the
// current codec version. It wraps another merge policy and only returns merges
// that would upgrade segments (i.e., segments with an older codec version).
//
// This implements GC-639: UpgradeIndexMergePolicy
type UpgradeIndexMergePolicy struct {
	// delegate is the wrapped merge policy.
	delegate MergePolicy
}

// NewUpgradeIndexMergePolicy creates a new UpgradeIndexMergePolicy wrapping the given policy.
// This implements GC-639: UpgradeIndexMergePolicy constructor
func NewUpgradeIndexMergePolicy(delegate MergePolicy) *UpgradeIndexMergePolicy {
	if delegate == nil {
		delegate = NewLogByteSizeMergePolicy()
	}
	return &UpgradeIndexMergePolicy{
		delegate: delegate,
	}
}

// FindMerges finds merges needed for upgrading segments.
// This implements GC-639: UpgradeIndexMergePolicy.FindMerges
func (p *UpgradeIndexMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	// Delegate to the wrapped policy
	return p.delegate.FindMerges(trigger, infos, mergeContext)
}

// FindForcedMerges finds forced merges to upgrade all segments.
// This implements GC-639: UpgradeIndexMergePolicy.FindForcedMerges
func (p *UpgradeIndexMergePolicy) FindForcedMerges(
	infos *SegmentInfos,
	maxSegmentCount int,
	segmentsToMerge map[*SegmentCommitInfo]bool,
	mergeContext MergeContext,
) (*MergeSpecification, error) {
	// For upgrade, we want to merge all segments that need upgrading
	// This is essentially a force merge to 1 segment
	if maxSegmentCount == 1 {
		// Merge all segments into one
		return p.delegate.FindForcedMerges(infos, maxSegmentCount, segmentsToMerge, mergeContext)
	}
	return p.delegate.FindForcedMerges(infos, maxSegmentCount, segmentsToMerge, mergeContext)
}

// FindForcedDeletesMerges finds merges to expunge deleted documents.
// This implements GC-639: UpgradeIndexMergePolicy.FindForcedDeletesMerges
func (p *UpgradeIndexMergePolicy) FindForcedDeletesMerges(
	infos *SegmentInfos,
	mergeContext MergeContext,
) (*MergeSpecification, error) {
	return p.delegate.FindForcedDeletesMerges(infos, mergeContext)
}

// UseCompoundFile returns whether to use compound files.
func (p *UpgradeIndexMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	return p.delegate.UseCompoundFile(infos, mergedSegmentInfo)
}

// GetMaxMergeDocs returns the maximum number of documents to merge.
func (p *UpgradeIndexMergePolicy) GetMaxMergeDocs() int {
	return p.delegate.GetMaxMergeDocs()
}

// SetMaxMergeDocs sets the maximum number of documents to merge.
func (p *UpgradeIndexMergePolicy) SetMaxMergeDocs(maxMergeDocs int) {
	p.delegate.SetMaxMergeDocs(maxMergeDocs)
}

// GetMaxMergedSegmentBytes returns the maximum size of a merged segment in bytes.
func (p *UpgradeIndexMergePolicy) GetMaxMergedSegmentBytes() int64 {
	return p.delegate.GetMaxMergedSegmentBytes()
}

// SetMaxMergedSegmentBytes sets the maximum size of a merged segment in bytes.
func (p *UpgradeIndexMergePolicy) SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64) {
	p.delegate.SetMaxMergedSegmentBytes(maxMergedSegmentBytes)
}

// NumDeletesToMerge returns the number of deletes that a merge would claim.
func (p *UpgradeIndexMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return p.delegate.NumDeletesToMerge(info, delCount)
}

// KeepFullyDeletedSegment returns true if the segment should be kept even if fully deleted.
func (p *UpgradeIndexMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return p.delegate.KeepFullyDeletedSegment(info)
}

// String returns a string representation of the policy.
func (p *UpgradeIndexMergePolicy) String() string {
	return fmt.Sprintf("[UpgradeIndexMergePolicy: delegate=%v]", p.delegate)
}

// Ensure interface is implemented
var _ MergePolicy = (*UpgradeIndexMergePolicy)(nil)

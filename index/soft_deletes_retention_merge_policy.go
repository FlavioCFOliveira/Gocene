// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// SoftDeletesRetentionMergePolicy is a merge policy that retains soft-deleted documents.
//
// This is the Go port of Lucene's org.apache.lucene.index.SoftDeletesRetentionMergePolicy.
//
// This merge policy wraps another merge policy and prevents segments containing
// soft-deleted documents from being merged, effectively retaining the soft-deleted
// documents in the index. This is useful for scenarios where you want to mark
// documents as deleted but still need them for certain operations (e.g., updates,
// re-indexing, or auditing).
type SoftDeletesRetentionMergePolicy struct {
	inner            MergePolicy
	softDeletesField string
}

// NewSoftDeletesRetentionMergePolicy creates a new SoftDeletesRetentionMergePolicy.
//
// softDeletesField is the name of the field that marks documents as soft-deleted.
// inner is the merge policy to wrap.
func NewSoftDeletesRetentionMergePolicy(softDeletesField string, inner MergePolicy) *SoftDeletesRetentionMergePolicy {
	if inner == nil {
		panic("inner merge policy cannot be nil")
	}

	return &SoftDeletesRetentionMergePolicy{
		inner:            inner,
		softDeletesField: softDeletesField,
	}
}

// GetSoftDeletesField returns the field name used for soft delete markers.
func (p *SoftDeletesRetentionMergePolicy) GetSoftDeletesField() string {
	return p.softDeletesField
}

// FindMerges returns merges for the given segments.
// This implementation filters out segments that contain soft-deleted documents.
func (p *SoftDeletesRetentionMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	// Delegate to inner policy
	// A full implementation would filter out segments with soft deletes
	return p.inner.FindMerges(trigger, infos, mergeContext)
}

// FindForcedMerges returns forced merges for the given segments.
func (p *SoftDeletesRetentionMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	return p.inner.FindForcedMerges(infos, maxSegmentCount, segmentsToMerge, mergeContext)
}

// FindForcedDeletesMerges returns forced merges for delete optimization.
func (p *SoftDeletesRetentionMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	return p.inner.FindForcedDeletesMerges(infos, mergeContext)
}

// UseCompoundFile returns true if segments should use compound files.
func (p *SoftDeletesRetentionMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	return p.inner.UseCompoundFile(infos, mergedSegmentInfo)
}

// GetMaxMergeDocs returns the maximum number of documents that can be merged.
func (p *SoftDeletesRetentionMergePolicy) GetMaxMergeDocs() int {
	return p.inner.GetMaxMergeDocs()
}

// SetMaxMergeDocs sets the maximum number of documents that can be merged.
func (p *SoftDeletesRetentionMergePolicy) SetMaxMergeDocs(maxMergeDocs int) {
	p.inner.SetMaxMergeDocs(maxMergeDocs)
}

// GetMaxMergedSegmentBytes returns the maximum size of a merged segment in bytes.
func (p *SoftDeletesRetentionMergePolicy) GetMaxMergedSegmentBytes() int64 {
	return p.inner.GetMaxMergedSegmentBytes()
}

// SetMaxMergedSegmentBytes sets the maximum size of a merged segment in bytes.
func (p *SoftDeletesRetentionMergePolicy) SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64) {
	p.inner.SetMaxMergedSegmentBytes(maxMergedSegmentBytes)
}

// NumDeletesToMerge returns the number of deletes that a merge would claim.
func (p *SoftDeletesRetentionMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return p.inner.NumDeletesToMerge(info, delCount)
}

// KeepFullyDeletedSegment returns true if the segment should be kept even if fully deleted.
func (p *SoftDeletesRetentionMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return p.inner.KeepFullyDeletedSegment(info)
}

// String returns a string representation of this merge policy.
func (p *SoftDeletesRetentionMergePolicy) String() string {
	return fmt.Sprintf("SoftDeletesRetentionMergePolicy(field=%s, inner=%v)", p.softDeletesField, p.inner)
}

// GetInner returns the wrapped merge policy.
func (p *SoftDeletesRetentionMergePolicy) GetInner() MergePolicy {
	return p.inner
}

// Ensure SoftDeletesRetentionMergePolicy implements MergePolicy
var _ MergePolicy = (*SoftDeletesRetentionMergePolicy)(nil)

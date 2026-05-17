// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FilterMergePolicy is the canonical pass-through wrapper around another
// MergePolicy. Subclasses (typically value types embedding *FilterMergePolicy)
// override only the methods they wish to intercept. Mirrors
// org.apache.lucene.index.FilterMergePolicy from Apache Lucene 10.4.0.
//
// Gocene-specific notes:
//   - All MergePolicy methods are forwarded to In without modification.
//   - Unwrap returns In so call-sites can introspect the wrapping chain
//     (Lucene's Unwrappable<MergePolicy> contract).
type FilterMergePolicy struct {
	In MergePolicy
}

// NewFilterMergePolicy wraps in. Panics if in is nil to surface the
// programming error early (matches Lucene's Objects.requireNonNull).
func NewFilterMergePolicy(in MergePolicy) *FilterMergePolicy {
	if in == nil {
		panic("FilterMergePolicy: in must not be nil")
	}
	return &FilterMergePolicy{In: in}
}

// Unwrap returns the underlying policy.
func (f *FilterMergePolicy) Unwrap() MergePolicy { return f.In }

// FindMerges delegates to In.
func (f *FilterMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	return f.In.FindMerges(trigger, infos, mc)
}

// FindForcedMerges delegates to In.
func (f *FilterMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segs map[*SegmentCommitInfo]bool, mc MergeContext) (*MergeSpecification, error) {
	return f.In.FindForcedMerges(infos, maxSegmentCount, segs, mc)
}

// FindForcedDeletesMerges delegates to In.
func (f *FilterMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	return f.In.FindForcedDeletesMerges(infos, mc)
}

// UseCompoundFile delegates to In.
func (f *FilterMergePolicy) UseCompoundFile(infos *SegmentInfos, merged *SegmentInfo) bool {
	return f.In.UseCompoundFile(infos, merged)
}

// GetMaxMergeDocs delegates to In.
func (f *FilterMergePolicy) GetMaxMergeDocs() int { return f.In.GetMaxMergeDocs() }

// SetMaxMergeDocs delegates to In.
func (f *FilterMergePolicy) SetMaxMergeDocs(n int) { f.In.SetMaxMergeDocs(n) }

// GetMaxMergedSegmentBytes delegates to In.
func (f *FilterMergePolicy) GetMaxMergedSegmentBytes() int64 { return f.In.GetMaxMergedSegmentBytes() }

// SetMaxMergedSegmentBytes delegates to In.
func (f *FilterMergePolicy) SetMaxMergedSegmentBytes(b int64) { f.In.SetMaxMergedSegmentBytes(b) }

// NumDeletesToMerge delegates to In.
func (f *FilterMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return f.In.NumDeletesToMerge(info, delCount)
}

// KeepFullyDeletedSegment delegates to In.
func (f *FilterMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return f.In.KeepFullyDeletedSegment(info)
}

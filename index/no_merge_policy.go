// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// NoMergePolicy is a merge policy that never selects any merges.
//
// This is the Go port of Lucene's org.apache.lucene.index.NoMergePolicy.
//
// This merge policy is useful for testing or when you want to disable
// all automatic merging. Note that this policy still allows explicit
// calls to forceMerge.
//
// This implements GC-640: NoMergePolicy
type NoMergePolicy struct {
	*BaseMergePolicy
}

// NewNoMergePolicy creates a new NoMergePolicy.
// This implements GC-640: NoMergePolicy constructor
func NewNoMergePolicy() *NoMergePolicy {
	return &NoMergePolicy{
		BaseMergePolicy: NewBaseMergePolicy(),
	}
}

// FindMerges never returns any merges.
// This implements GC-640: NoMergePolicy.FindMerges
func (p *NoMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	// Never return any merges
	return nil, nil
}

// FindForcedMerges finds forced merges.
// This implements GC-640: NoMergePolicy.FindForcedMerges
func (p *NoMergePolicy) FindForcedMerges(
	infos *SegmentInfos,
	maxSegmentCount int,
	segmentsToMerge map[*SegmentCommitInfo]bool,
	mergeContext MergeContext,
) (*MergeSpecification, error) {
	// For forced merges, we still need to honor the request
	// but we do it one segment at a time
	if maxSegmentCount <= 0 {
		return nil, nil
	}

	if infos.Size() <= maxSegmentCount {
		return nil, nil
	}

	// Simple implementation: merge segments one at a time
	// until we reach the desired count
	spec := NewMergeSpecification()

	// Collect segments to merge
	segments := make([]*SegmentCommitInfo, 0, infos.Size())
	for sci := range infos.Iterator() {
		if _, shouldMerge := segmentsToMerge[sci]; shouldMerge {
			segments = append(segments, sci)
		}
	}

	// Merge until we have maxSegmentCount or fewer
	for len(segments) > maxSegmentCount {
		// Take up to 2 segments at a time
		end := 2
		if end > len(segments) {
			end = len(segments)
		}
		if end < 2 {
			break
		}

		merge := NewOneMerge(segments[:end])
		spec.Add(merge)
		segments = segments[end:]
	}

	if spec.Size() == 0 {
		return nil, nil
	}

	return spec, nil
}

// FindForcedDeletesMerges never returns any merges.
// This implements GC-640: NoMergePolicy.FindForcedDeletesMerges
func (p *NoMergePolicy) FindForcedDeletesMerges(
	infos *SegmentInfos,
	mergeContext MergeContext,
) (*MergeSpecification, error) {
	// Never return any merges
	return nil, nil
}

// UseCompoundFile returns whether to use compound files.
func (p *NoMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	// Use the default behavior
	return false
}

// NumDeletesToMerge returns the number of deletes for a segment.
func (p *NoMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return delCount
}

// KeepFullyDeletedSegment returns false by default.
func (p *NoMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return false
}

// String returns a string representation of the policy.
func (p *NoMergePolicy) String() string {
	return "[NoMergePolicy]"
}

// Ensure interface is implemented
var _ MergePolicy = (*NoMergePolicy)(nil)

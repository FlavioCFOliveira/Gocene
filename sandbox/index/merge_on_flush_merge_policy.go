// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.index.MergeOnFlushMergePolicy.
package index

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// defaultSmallSegmentThresholdMB is the default threshold for "small" segments
// (100 MB). Mirrors MergeOnFlushMergePolicy.smallSegmentThresholdBytes default.
const defaultSmallSegmentThresholdMB = 100.0

// MergeOnFlushMergePolicy wraps a MergePolicy and merges all small segments
// (smaller than SmallSegmentThresholdMB) into one segment on commit.
//
// Mirrors org.apache.lucene.sandbox.index.MergeOnFlushMergePolicy.
type MergeOnFlushMergePolicy struct {
	*index.FilterMergePolicy

	// smallSegmentThresholdBytes is the threshold below which a segment is
	// considered "small" and eligible for merging on flush. Default 100 MB.
	smallSegmentThresholdBytes int64
}

// NewMergeOnFlushMergePolicy wraps the given MergePolicy. The default small
// segment threshold is 100 MB.
//
// Mirrors MergeOnFlushMergePolicy(MergePolicy).
func NewMergeOnFlushMergePolicy(mergePolicy index.MergePolicy) *MergeOnFlushMergePolicy {
	return &MergeOnFlushMergePolicy{
		FilterMergePolicy:          index.NewFilterMergePolicy(mergePolicy),
		smallSegmentThresholdBytes: MergeOnFlushUnits.MBToBytes(defaultSmallSegmentThresholdMB),
	}
}

// GetSmallSegmentThresholdMB returns the threshold in megabytes.
func (p *MergeOnFlushMergePolicy) GetSmallSegmentThresholdMB() float64 {
	return MergeOnFlushUnits.BytesToMB(p.smallSegmentThresholdBytes)
}

// SetSmallSegmentThresholdMB sets the threshold. All segments smaller than
// this will be merged into a single segment before commit completes.
func (p *MergeOnFlushMergePolicy) SetSmallSegmentThresholdMB(mb float64) {
	p.smallSegmentThresholdBytes = MergeOnFlushUnits.MBToBytes(mb)
}

// FindFullFlushMerges selects small segments for merging at commit time.
// Returns a MergeSpecification that merges all small segments (those with
// sizeInBytes < smallSegmentThresholdBytes) that are not already participating
// in a merge. Returns nil when there are fewer than 2 eligible segments.
//
// Mirrors MergeOnFlushMergePolicy.findFullFlushMerges(MergeTrigger,
// SegmentInfos, MergeContext).
func (p *MergeOnFlushMergePolicy) FindFullFlushMerges(
	_ index.MergeTrigger,
	segmentInfos *index.SegmentInfos,
	mergeContext index.MergeContext,
) (*index.MergeSpecification, error) {
	var smallSegments []*index.SegmentCommitInfo
	mergingSegs := mergeContext.GetMergingSegments()
	for i := 0; i < segmentInfos.Size(); i++ {
		sci := segmentInfos.Get(i)
		if sci.SegmentInfo().SizeInBytes() < p.smallSegmentThresholdBytes {
			if !mergingSegs[sci] {
				smallSegments = append(smallSegments, sci)
			}
		}
	}
	if len(smallSegments) > 1 {
		spec := index.NewMergeSpecification()
		spec.Add(index.NewOneMerge(smallSegments))
		return spec, nil
	}
	return nil, nil
}

// MergeOnFlushUnits provides conversion utilities between megabytes and bytes.
// Mirrors MergeOnFlushMergePolicy.Units.
var MergeOnFlushUnits mergeOnFlushUnits

// mergeOnFlushUnits is the value type for the MergeOnFlushUnits singleton.
type mergeOnFlushUnits struct{}

// BytesToMB converts bytes to megabytes.
func (mergeOnFlushUnits) BytesToMB(bytes int64) float64 {
	return float64(bytes) / 1024.0 / 1024.0
}

// MBToBytes converts megabytes to bytes.
func (mergeOnFlushUnits) MBToBytes(megabytes float64) int64 {
	return int64(megabytes * 1024 * 1024)
}

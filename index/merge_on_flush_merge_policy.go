// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// MergeOnFlushMergePolicy is a merge policy that merges all tiny segments
// (smaller than a specified threshold) into one segment on flush.
//
// This is the Go port of Lucene's org.apache.lucene.sandbox.index.MergeOnFlushMergePolicy.
type MergeOnFlushMergePolicy struct {
	*FilterMergePolicy
	smallSegmentThresholdBytes int64
}

// NewMergeOnFlushMergePolicy creates a new MergeOnFlushMergePolicy wrapping another.
func NewMergeOnFlushMergePolicy(in MergePolicy) *MergeOnFlushMergePolicy {
	return &MergeOnFlushMergePolicy{
		FilterMergePolicy:          NewFilterMergePolicy(in),
		smallSegmentThresholdBytes: mbToBytes(100.0),
	}
}

// GetSmallSegmentThresholdMB returns the threshold in megabytes.
func (m *MergeOnFlushMergePolicy) GetSmallSegmentThresholdMB() float64 {
	return bytesToMB(m.smallSegmentThresholdBytes)
}

// SetSmallSegmentThresholdMB sets the threshold for small segments in megabytes.
func (m *MergeOnFlushMergePolicy) SetSmallSegmentThresholdMB(mb float64) {
	m.smallSegmentThresholdBytes = mbToBytes(mb)
}

// FindFullFlushMerges identifies merges of tiny segments that should occur on flush.
func (m *MergeOnFlushMergePolicy) FindFullFlushMerges(trigger MergeTrigger, infos *spi.SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	var smallSegments []SegmentCommitInfo

	for _, sci := range infos.Segments() {
		if sci.SegmentInfo().SizeInBytes() < m.smallSegmentThresholdBytes {
			if !mc.GetMergingSegments()[sci] {
				smallSegments = append(smallSegments, *sci)
			}
		}
	}

	if len(smallSegments) > 1 {
		spec := &MergeSpecification{}
		spec.Add(NewOneMerge(smallSegments))
		return spec, nil
	}

	return nil, nil
}

func bytesToMB(bytes int64) float64 {
	return float64(bytes) / 1024.0 / 1024.0
}

func mbToBytes(mb float64) int64 {
	return int64(mb * 1024 * 1024)
}

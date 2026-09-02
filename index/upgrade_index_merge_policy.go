// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// UpgradeIndexMergePolicy is a MergePolicy used for upgrading all existing
// segments of an index when calling IndexWriter.ForceMerge. All other methods
// delegate to the base MergePolicy given to the constructor. This allows for
// an as-cheap-as-possible upgrade of an older index by only upgrading segments
// that were created by previous Lucene versions; forceMerge no longer really
// merges — it is just used to "forceMerge" older segment versions away.
//
// This is the Go port of org.apache.lucene.index.UpgradeIndexMergePolicy
// from Apache Lucene 10.4.0.
type UpgradeIndexMergePolicy struct {
	// delegate is the wrapped merge policy.
	delegate MergePolicy
}

// NewUpgradeIndexMergePolicy wraps the given delegate MergePolicy.
// If delegate is nil, NewTieredMergePolicy is used as the default.
func NewUpgradeIndexMergePolicy(delegate MergePolicy) *UpgradeIndexMergePolicy {
	if delegate == nil {
		delegate = NewTieredMergePolicy()
	}
	return &UpgradeIndexMergePolicy{delegate: delegate}
}

// shouldUpgradeSegment returns true if the segment was written by an older
// Lucene version. The check mirrors Version.LATEST.equals(si.info.getVersion()):
// any version string other than the current release requires rewriting.
func (p *UpgradeIndexMergePolicy) shouldUpgradeSegment(sci *SegmentCommitInfo) bool {
	si := sci.SegmentInfo()
	if si == nil {
		return true
	}
	v := si.Version()
	if v == "" {
		return true // unknown version always needs upgrade
	}
	latest := fmt.Sprintf("%d.%d.%d",
		util.LuceneVersionMajor, util.LuceneVersionMinor, util.LuceneVersionBugfix)
	return v != latest
}

// FindMerges delegates to the wrapped policy for background merges.
// Mirrors UpgradeIndexMergePolicy.findMerges which calls
// in.findMerges(null, segmentInfos, mergeContext).
func (p *UpgradeIndexMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	return p.delegate.FindMerges(SEGMENT_FLUSH, infos, mc)
}

// FindForcedMerges implements the upgrade logic: only segments that need
// upgrading are included. If the delegate leaves any old segments untouched
// they are collected into one additional merge. Mirrors
// UpgradeIndexMergePolicy.findForcedMerges exactly.
func (p *UpgradeIndexMergePolicy) FindForcedMerges(
	infos *SegmentInfos,
	maxSegmentCount int,
	segmentsToMerge map[*SegmentCommitInfo]bool,
	mc MergeContext,
) (*MergeSpecification, error) {
	// Collect only segments that are both requested and need upgrading.
	oldSegments := make(map[*SegmentCommitInfo]bool)
	for sci, include := range segmentsToMerge {
		if include && p.shouldUpgradeSegment(sci) {
			oldSegments[sci] = include
		}
	}

	if len(oldSegments) == 0 {
		return nil, nil
	}

	spec, err := p.delegate.FindForcedMerges(infos, maxSegmentCount, oldSegments, mc)
	if err != nil {
		return nil, err
	}

	// Remove segments already covered by the delegate's specification.
	if spec != nil {
		for _, om := range spec.Merges {
			for _, sci := range om.Segments {
				delete(oldSegments, sci)
			}
		}
	}

	// Any remaining old segments are merged into one additional merge.
	if len(oldSegments) > 0 {
		newInfos := make([]*SegmentCommitInfo, 0, len(oldSegments))
		for _, sci := range infos.List() {
			if oldSegments[sci] {
				newInfos = append(newInfos, sci)
			}
		}
		if len(newInfos) > 0 {
			if spec == nil {
				spec = NewMergeSpecification()
			}
			spec.Add(NewOneMerge(newInfos))
		}
	}

	return spec, nil
}

// FindForcedDeletesMerges delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) FindForcedDeletesMerges(
	infos *SegmentInfos,
	mc MergeContext,
) (*MergeSpecification, error) {
	return p.delegate.FindForcedDeletesMerges(infos, mc)
}

// UseCompoundFile delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	return p.delegate.UseCompoundFile(infos, mergedSegmentInfo)
}

// GetMaxMergeDocs delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) GetMaxMergeDocs() int {
	return p.delegate.GetMaxMergeDocs()
}

// SetMaxMergeDocs delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) SetMaxMergeDocs(maxMergeDocs int) {
	p.delegate.SetMaxMergeDocs(maxMergeDocs)
}

// GetMaxMergedSegmentBytes delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) GetMaxMergedSegmentBytes() int64 {
	return p.delegate.GetMaxMergedSegmentBytes()
}

// SetMaxMergedSegmentBytes delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64) {
	p.delegate.SetMaxMergedSegmentBytes(maxMergedSegmentBytes)
}

// NumDeletesToMerge delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return p.delegate.NumDeletesToMerge(info, delCount)
}

// KeepFullyDeletedSegment delegates to the wrapped policy.
func (p *UpgradeIndexMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return p.delegate.KeepFullyDeletedSegment(info)
}

// String returns a string representation of the policy.
func (p *UpgradeIndexMergePolicy) String() string {
	return fmt.Sprintf("[UpgradeIndexMergePolicy: delegate=%v]", p.delegate)
}

// Compile-time assertion that UpgradeIndexMergePolicy satisfies MergePolicy.
var _ MergePolicy = (*UpgradeIndexMergePolicy)(nil)

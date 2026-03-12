// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"
)

// TieredMergePolicy merges segments of similar size.
// This is the default merge policy for Lucene and the Go port.
//
// TieredMergePolicy groups segments into tiers based on their size.
// Each tier contains segments within a factor of each other (controlled by tierExponent).
// When a tier has enough segments, they are merged together.
//
// This policy is generally superior to LogMergePolicy because it:
//   - Produces more balanced segment sizes
//   - Requires less total merging work
//   - Handles varying indexing rates better
//
// This is the Go port of Lucene's org.apache.lucene.index.TieredMergePolicy.
type TieredMergePolicy struct {
	*BaseMergePolicy

	// maxMergeAtOnce is the maximum number of segments to merge at once (default 10)
	maxMergeAtOnce int

	// maxMergeAtOnceExplicit is the maximum number of segments to merge at once
	// for explicit merges (default 30)
	maxMergeAtOnceExplicit int

	// maxMergedSegmentMB is the maximum size of a merged segment in MB (default 5120 = 5GB)
	maxMergedSegmentMB int64

	// floorSegmentMB is the minimum segment size to consider for merging in MB (default 2)
	// Segments smaller than this are treated as this size
	floorSegmentMB int64

	// tierExponent controls the tier boundary (default 0.5 = sqrt)
	// Higher values make tiers more spaced out
	tierExponent float64

	// noCFSRatio is the ratio of non-compound file segments (default 0.1)
	// If the merged segment size / total index size < this ratio, use CFS
	noCFSRatio float64

	// deletesPctAllowed is the percentage of deleted documents allowed (default 33.0)
	deletesPctAllowed float64
}

// NewTieredMergePolicy creates a new TieredMergePolicy with default settings.
func NewTieredMergePolicy() *TieredMergePolicy {
	return &TieredMergePolicy{
		BaseMergePolicy:        NewBaseMergePolicy(),
		maxMergeAtOnce:         10,
		maxMergeAtOnceExplicit: 30,
		maxMergedSegmentMB:     5120, // 5GB
		floorSegmentMB:         2,
		tierExponent:           0.5,
		noCFSRatio:             0.1,
		deletesPctAllowed:      33.0,
	}
}

// GetMaxMergeAtOnce returns the maximum number of segments to merge at once.
func (p *TieredMergePolicy) GetMaxMergeAtOnce() int {
	return p.maxMergeAtOnce
}

// SetMaxMergeAtOnce sets the maximum number of segments to merge at once.
func (p *TieredMergePolicy) SetMaxMergeAtOnce(maxMergeAtOnce int) {
	p.maxMergeAtOnce = maxMergeAtOnce
}

// GetMaxMergeAtOnceExplicit returns the maximum number of segments to merge at once for explicit merges.
func (p *TieredMergePolicy) GetMaxMergeAtOnceExplicit() int {
	return p.maxMergeAtOnceExplicit
}

// SetMaxMergeAtOnceExplicit sets the maximum number of segments to merge at once for explicit merges.
func (p *TieredMergePolicy) SetMaxMergeAtOnceExplicit(maxMergeAtOnceExplicit int) {
	p.maxMergeAtOnceExplicit = maxMergeAtOnceExplicit
}

// GetMaxMergedSegmentMB returns the maximum size of a merged segment in MB.
func (p *TieredMergePolicy) GetMaxMergedSegmentMB() int64 {
	return p.maxMergedSegmentMB
}

// SetMaxMergedSegmentMB sets the maximum size of a merged segment in MB.
func (p *TieredMergePolicy) SetMaxMergedSegmentMB(maxMergedSegmentMB int64) {
	p.maxMergedSegmentMB = maxMergedSegmentMB
	p.SetMaxMergedSegmentBytes(mbToBytes(maxMergedSegmentMB))
}

// GetFloorSegmentMB returns the minimum segment size to consider for merging in MB.
func (p *TieredMergePolicy) GetFloorSegmentMB() int64 {
	return p.floorSegmentMB
}

// SetFloorSegmentMB sets the minimum segment size to consider for merging in MB.
func (p *TieredMergePolicy) SetFloorSegmentMB(floorSegmentMB int64) {
	p.floorSegmentMB = floorSegmentMB
}

// GetTierExponent returns the tier exponent.
func (p *TieredMergePolicy) GetTierExponent() float64 {
	return p.tierExponent
}

// SetTierExponent sets the tier exponent.
func (p *TieredMergePolicy) SetTierExponent(tierExponent float64) {
	p.tierExponent = tierExponent
}

// GetNoCFSRatio returns the ratio of non-compound file segments.
func (p *TieredMergePolicy) GetNoCFSRatio() float64 {
	return p.noCFSRatio
}

// SetNoCFSRatio sets the ratio of non-compound file segments.
func (p *TieredMergePolicy) SetNoCFSRatio(noCFSRatio float64) {
	p.noCFSRatio = noCFSRatio
}

// GetDeletesPctAllowed returns the percentage of deleted documents allowed.
func (p *TieredMergePolicy) GetDeletesPctAllowed() float64 {
	return p.deletesPctAllowed
}

// SetDeletesPctAllowed sets the percentage of deleted documents allowed.
func (p *TieredMergePolicy) SetDeletesPctAllowed(deletesPctAllowed float64) {
	p.deletesPctAllowed = deletesPctAllowed
}

// FindMerges finds merges based on tiered policy.
func (p *TieredMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos) (*MergeSpecification, error) {
	if infos.Size() < 2 {
		return nil, nil
	}

	// Get all segments sorted by size
	segments := p.getSortedSegments(infos)
	if len(segments) == 0 {
		return nil, nil
	}

	// Check if there are too many segments
	spec := p.findMergesForTier(segments, p.maxMergeAtOnce)
	if spec != nil && spec.Size() > 0 {
		return spec, nil
	}

	return nil, nil
}

// FindForcedMerges finds forced merges (for optimize).
func (p *TieredMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int) (*MergeSpecification, error) {
	if infos.Size() <= maxSegmentCount {
		return nil, nil
	}

	segments := p.getSortedSegments(infos)
	if len(segments) == 0 {
		return nil, nil
	}

	// Merge segments to reach maxSegmentCount
	spec := NewMergeSpecification()
	maxMergeAtOnce := p.maxMergeAtOnceExplicit

	// Calculate how many merges we need
	numSegments := len(segments)
	for numSegments > maxSegmentCount {
		// Take the smallest segments first
		end := maxMergeAtOnce
		if end > numSegments {
			end = numSegments
		}

		// For the final merge, ensure we don't exceed maxSegmentCount
		if numSegments-end+1 > maxSegmentCount {
			// Merge end segments
			mergeSegments := make([]*SegmentCommitInfo, end)
			copy(mergeSegments, segments[:end])
			spec.Add(NewOneMerge(mergeSegments))
			segments = segments[end:]
		} else {
			// Final merge: merge enough to reach target
			toMerge := numSegments - maxSegmentCount + 1
			mergeSegments := make([]*SegmentCommitInfo, toMerge)
			copy(mergeSegments, segments[:toMerge])
			spec.Add(NewOneMerge(mergeSegments))
			segments = segments[toMerge:]
		}

		numSegments = len(segments)
	}

	if spec.Size() > 0 {
		return spec, nil
	}
	return nil, nil
}

// FindForcedDeletesMerges finds merges to expunge deleted documents.
func (p *TieredMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos) (*MergeSpecification, error) {
	// Find segments with high delete ratios
	segments := make([]*SegmentCommitInfo, 0)

	for sci := range infos.Iterator() {
		docCount := sci.DocCount()
		if docCount == 0 {
			continue
		}

		delCount := sci.DelCount()
		deleteRatio := float64(delCount) / float64(docCount) * 100.0

		// If delete ratio exceeds threshold, consider for merge
		if deleteRatio > p.deletesPctAllowed {
			segments = append(segments, sci)
		}
	}

	if len(segments) == 0 {
		return nil, nil
	}

	// Sort by delete ratio (highest first)
	sort.Slice(segments, func(i, j int) bool {
		ratioI := float64(segments[i].DelCount()) / float64(segments[i].DocCount())
		ratioJ := float64(segments[j].DelCount()) / float64(segments[j].DocCount())
		return ratioI > ratioJ
	})

	// Create merges for segments with high deletes
	spec := NewMergeSpecification()
	maxMergeAtOnce := p.maxMergeAtOnce

	for len(segments) > 0 {
		toMerge := maxMergeAtOnce
		if toMerge > len(segments) {
			toMerge = len(segments)
		}

		mergeSegments := make([]*SegmentCommitInfo, toMerge)
		copy(mergeSegments, segments[:toMerge])
		spec.Add(NewOneMerge(mergeSegments))
		segments = segments[toMerge:]
	}

	if spec.Size() > 0 {
		return spec, nil
	}
	return nil, nil
}

// UseCompoundFile returns true if segments should use compound files.
func (p *TieredMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	if p.noCFSRatio >= 1.0 {
		return false
	}

	if p.noCFSRatio <= 0.0 {
		return true
	}

	// Calculate total index size
	totalSize := int64(0)
	for sci := range infos.Iterator() {
		totalSize += sci.SegmentInfo().SizeInBytes()
	}

	if totalSize == 0 {
		return true
	}

	// Use CFS if merged segment is small relative to total
	mergedSize := mergedSegmentInfo.SizeInBytes()
	ratio := float64(mergedSize) / float64(totalSize)

	return ratio < p.noCFSRatio
}

// getSortedSegments returns segments sorted by size (smallest first).
func (p *TieredMergePolicy) getSortedSegments(infos *SegmentInfos) []*SegmentCommitInfo {
	segments := make([]*SegmentCommitInfo, 0, infos.Size())

	for sci := range infos.Iterator() {
		segments = append(segments, sci)
	}

	// Sort by size (estimated from doc count and file size)
	sort.Slice(segments, func(i, j int) bool {
		sizeI := segments[i].SegmentInfo().SizeInBytes()
		sizeJ := segments[j].SegmentInfo().SizeInBytes()

		// Apply floor segment size
		if sizeI < mbToBytes(p.floorSegmentMB) {
			sizeI = mbToBytes(p.floorSegmentMB)
		}
		if sizeJ < mbToBytes(p.floorSegmentMB) {
			sizeJ = mbToBytes(p.floorSegmentMB)
		}

		return sizeI < sizeJ
	})

	return segments
}

// findMergesForTier finds merges for segments grouped by tier.
func (p *TieredMergePolicy) findMergesForTier(segments []*SegmentCommitInfo, maxMergeAtOnce int) *MergeSpecification {
	if len(segments) < 2 {
		return nil
	}

	// Group segments by tier
	tierGroups := p.groupByTier(segments)

	spec := NewMergeSpecification()

	// For each tier, check if we have enough segments to merge
	for _, tier := range tierGroups {
		if len(tier) >= maxMergeAtOnce {
			// Found a tier with enough segments - merge them
			toMerge := maxMergeAtOnce
			if toMerge > len(tier) {
				toMerge = len(tier)
			}

			// Check if merge would exceed max merged segment size
			var mergeSize int64
			for i := 0; i < toMerge; i++ {
				mergeSize += tier[i].SegmentInfo().SizeInBytes()
			}

			if mergeSize <= mbToBytes(p.maxMergedSegmentMB) {
				mergeSegments := make([]*SegmentCommitInfo, toMerge)
				copy(mergeSegments, tier[:toMerge])
				spec.Add(NewOneMerge(mergeSegments))
				return spec
			}
		}
	}

	if spec.Size() > 0 {
		return spec
	}
	return nil
}

// groupByTier groups segments by tier based on their size.
func (p *TieredMergePolicy) groupByTier(segments []*SegmentCommitInfo) [][]*SegmentCommitInfo {
	if len(segments) == 0 {
		return nil
	}

	// Calculate tier boundaries
	// Tier i contains segments where: tierMin <= size < tierMax
	// tierMax = tierMin * (2 ^ tierExponent)
	// With tierExponent = 0.5, tiers are: 0-2MB, 2-4MB, 4-8MB, etc.

	var tiers [][]*SegmentCommitInfo
	floorBytes := mbToBytes(p.floorSegmentMB)

	// Start with the smallest tier
	currentTierMax := float64(floorBytes)
	var currentTier []*SegmentCommitInfo

	for _, seg := range segments {
		size := seg.SegmentInfo().SizeInBytes()
		if size < floorBytes {
			size = floorBytes
		}

		// Check if this segment fits in the current tier
		for float64(size) >= currentTierMax {
			// Save current tier if it has segments
			if len(currentTier) > 0 {
				tiers = append(tiers, currentTier)
				currentTier = nil
			}

			// Move to next tier
			currentTierMax = currentTierMax * math.Pow(2, p.tierExponent)
		}

		currentTier = append(currentTier, seg)
	}

	// Add the last tier
	if len(currentTier) > 0 {
		tiers = append(tiers, currentTier)
	}

	return tiers
}

// sortBySegmentSize sorts segment infos by size.
func (p *TieredMergePolicy) sortBySegmentSize(segments []*SegmentCommitInfo) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].SegmentInfo().SizeInBytes() < segments[j].SegmentInfo().SizeInBytes()
	})
}

// String returns a string representation of the TieredMergePolicy.
func (p *TieredMergePolicy) String() string {
	return fmt.Sprintf("TieredMergePolicy(maxMergeAtOnce=%d, maxMergedSegmentMB=%d, floorSegmentMB=%d, tierExponent=%.2f)",
		p.maxMergeAtOnce, p.maxMergedSegmentMB, p.floorSegmentMB, p.tierExponent)
}

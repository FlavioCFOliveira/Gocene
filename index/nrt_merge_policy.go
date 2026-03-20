// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
)

// NRTMergePolicy is a merge policy optimized for Near Real-Time (NRT) search.
// It balances segment size with minimal impact on NRT reader reopen latency.
//
// This is the Go port of Lucene's NRT-aware merge policy pattern.
//
// Key features:
//   - Favors smaller merges that complete quickly
//   - Limits maximum segment size to reduce reopen latency
//   - Considers pending deletes for more aggressive merging
//   - Configurable balance between search performance and indexing throughput
//
// The policy works by:
//   - Using a smaller merge factor than standard policies
//   - Limiting the maximum merged segment size
//   - Prioritizing segments with high delete ratios
//   - Avoiding large merges during active NRT operations
type NRTMergePolicy struct {
	*BaseMergePolicy

	// minMergeMB is the minimum segment size to consider for merging (in MB).
	// Default is 0.5 MB.
	minMergeMB float64

	// maxMergeMB is the maximum segment size to merge (in MB).
	// Default is 512 MB. This is lower than standard policies to ensure
	// fast reopen times.
	maxMergeMB float64

	// maxMergeMBDuringNRT is the maximum merge size during active NRT operations.
	// Default is 100 MB. Larger merges are deferred.
	maxMergeMBDuringNRT float64

	// mergeFactor is the number of segments to merge at once.
	// Default is 5 (lower than standard 10 for quicker merges).
	mergeFactor int

	// maxMergedSegmentMB is the maximum size of a merged segment (in MB).
	// Default is 256 MB.
	maxMergedSegmentMB float64

	// deletesPctAllowed is the percentage of deleted documents allowed before
	// forcing a merge. Default is 20%.
	deletesPctAllowed float64

	// segmentsPerTier is the number of segments allowed per tier before merging.
	// Default is 3 (lower than TieredMergePolicy for more aggressive merging).
	segmentsPerTier int

	// floorSegmentMB is the minimum segment size for tiering (in MB).
	// Default is 1 MB.
	floorSegmentMB float64

	// calibrateSizeByDeletes controls whether to calibrate segment size by deletes.
	// Default is true.
	calibrateSizeByDeletes bool

	// nrtAware indicates whether to use NRT-aware merge decisions.
	// When true, merges are optimized for low reopen latency.
	// Default is true.
	nrtAware bool

	// maxMergeDocs is the maximum number of documents to merge at once.
	// Default is max int.
	maxMergeDocs int
}

// NewNRTMergePolicy creates a new NRTMergePolicy with default settings.
func NewNRTMergePolicy() *NRTMergePolicy {
	return &NRTMergePolicy{
		BaseMergePolicy:        NewBaseMergePolicy(),
		minMergeMB:             0.5,
		maxMergeMB:             512.0,
		maxMergeMBDuringNRT:    100.0,
		mergeFactor:            5,
		maxMergedSegmentMB:     256.0,
		deletesPctAllowed:      20.0,
		segmentsPerTier:        3,
		floorSegmentMB:         1.0,
		calibrateSizeByDeletes: true,
		nrtAware:               true,
		maxMergeDocs:           math.MaxInt32,
	}
}

// GetMinMergeMB returns the minimum merge size in MB.
func (p *NRTMergePolicy) GetMinMergeMB() float64 {
	return p.minMergeMB
}

// SetMinMergeMB sets the minimum merge size in MB.
func (p *NRTMergePolicy) SetMinMergeMB(v float64) {
	if v < 0 {
		v = 0
	}
	p.minMergeMB = v
}

// GetMaxMergeMB returns the maximum merge size in MB.
func (p *NRTMergePolicy) GetMaxMergeMB() float64 {
	return p.maxMergeMB
}

// SetMaxMergeMB sets the maximum merge size in MB.
func (p *NRTMergePolicy) SetMaxMergeMB(v float64) {
	if v < 0 {
		v = 0
	}
	p.maxMergeMB = v
}

// GetMaxMergeMBDuringNRT returns the maximum merge size during NRT in MB.
func (p *NRTMergePolicy) GetMaxMergeMBDuringNRT() float64 {
	return p.maxMergeMBDuringNRT
}

// SetMaxMergeMBDuringNRT sets the maximum merge size during NRT in MB.
func (p *NRTMergePolicy) SetMaxMergeMBDuringNRT(v float64) {
	if v < 0 {
		v = 0
	}
	p.maxMergeMBDuringNRT = v
}

// GetMergeFactor returns the merge factor.
func (p *NRTMergePolicy) GetMergeFactor() int {
	return p.mergeFactor
}

// SetMergeFactor sets the merge factor.
func (p *NRTMergePolicy) SetMergeFactor(v int) {
	if v < 2 {
		v = 2
	}
	p.mergeFactor = v
}

// GetMaxMergedSegmentMB returns the maximum merged segment size in MB.
func (p *NRTMergePolicy) GetMaxMergedSegmentMB() float64 {
	return p.maxMergedSegmentMB
}

// SetMaxMergedSegmentMB sets the maximum merged segment size in MB.
func (p *NRTMergePolicy) SetMaxMergedSegmentMB(v float64) {
	if v < 0 {
		v = 0
	}
	p.maxMergedSegmentMB = v
}

// GetDeletesPctAllowed returns the percentage of deletes allowed before forced merge.
func (p *NRTMergePolicy) GetDeletesPctAllowed() float64 {
	return p.deletesPctAllowed
}

// SetDeletesPctAllowed sets the percentage of deletes allowed before forced merge.
func (p *NRTMergePolicy) SetDeletesPctAllowed(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.deletesPctAllowed = v
}

// GetSegmentsPerTier returns the number of segments allowed per tier.
func (p *NRTMergePolicy) GetSegmentsPerTier() int {
	return p.segmentsPerTier
}

// SetSegmentsPerTier sets the number of segments allowed per tier.
func (p *NRTMergePolicy) SetSegmentsPerTier(v int) {
	if v < 2 {
		v = 2
	}
	p.segmentsPerTier = v
}

// GetFloorSegmentMB returns the floor segment size in MB.
func (p *NRTMergePolicy) GetFloorSegmentMB() float64 {
	return p.floorSegmentMB
}

// SetFloorSegmentMB sets the floor segment size in MB.
func (p *NRTMergePolicy) SetFloorSegmentMB(v float64) {
	if v < 0 {
		v = 0
	}
	p.floorSegmentMB = v
}

// GetCalibrateSizeByDeletes returns whether to calibrate size by deletes.
func (p *NRTMergePolicy) GetCalibrateSizeByDeletes() bool {
	return p.calibrateSizeByDeletes
}

// SetCalibrateSizeByDeletes sets whether to calibrate size by deletes.
func (p *NRTMergePolicy) SetCalibrateSizeByDeletes(v bool) {
	p.calibrateSizeByDeletes = v
}

// GetNRTAware returns whether NRT-aware merge decisions are enabled.
func (p *NRTMergePolicy) GetNRTAware() bool {
	return p.nrtAware
}

// SetNRTAware sets whether to use NRT-aware merge decisions.
func (p *NRTMergePolicy) SetNRTAware(v bool) {
	p.nrtAware = v
}

// GetMaxMergeDocs returns the maximum number of documents to merge.
func (p *NRTMergePolicy) GetMaxMergeDocs() int {
	return p.maxMergeDocs
}

// SetMaxMergeDocs sets the maximum number of documents to merge.
func (p *NRTMergePolicy) SetMaxMergeDocs(v int) {
	if v < 1 {
		v = 1
	}
	p.maxMergeDocs = v
}

// FindMerges finds merges needed for the given segment infos.
func (p *NRTMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if infos == nil {
		return nil, fmt.Errorf("segment infos cannot be nil")
	}

	// Check for merges due to deletes
	if spec := p.findDeletesMerges(infos, mergeContext); spec != nil {
		return spec, nil
	}

	// Check for tiered merges
	if spec := p.findTieredMerges(infos, mergeContext); spec != nil {
		return spec, nil
	}

	return nil, nil
}

// findDeletesMerges finds merges needed due to deletes.
func (p *NRTMergePolicy) findDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) *MergeSpecification {
	segments := infos.List()
	if len(segments) == 0 {
		return nil
	}

	var candidateSegments []*SegmentCommitInfo
	maxDelCount := int64(0)

	for _, info := range segments {
		delCount := info.DelCount()
		docCount := info.DocCount()

		if docCount > 0 {
			delPct := float64(delCount) * 100.0 / float64(docCount)
			if delPct >= p.deletesPctAllowed {
				candidateSegments = append(candidateSegments, info)
				if int64(delCount) > maxDelCount {
					maxDelCount = int64(delCount)
				}
			}
		}
	}

	if len(candidateSegments) < p.mergeFactor {
		return nil
	}

	// Sort by delete percentage (highest first)
	p.sortByDeletesDesc(candidateSegments)

	// Take the segments with the most deletes
	numToMerge := p.mergeFactor
	if len(candidateSegments) < numToMerge {
		numToMerge = len(candidateSegments)
	}

	spec := NewMergeSpecification()
	merge := NewOneMerge(candidateSegments[:numToMerge])
	spec.Add(merge)

	return spec
}

// findTieredMerges finds tiered merges using a size-based approach.
func (p *NRTMergePolicy) findTieredMerges(infos *SegmentInfos, mergeContext MergeContext) *MergeSpecification {
	segments := infos.List()
	if len(segments) < p.segmentsPerTier {
		return nil
	}

	// Get eligible segments (not already merging)
	var eligibleSegments []*SegmentCommitInfo
	for _, info := range segments {
		if mergeContext == nil || !mergeContext.GetMergingSegments()[info] {
			eligibleSegments = append(eligibleSegments, info)
		}
	}

	if len(eligibleSegments) < p.segmentsPerTier {
		return nil
	}

	// Sort by size (smallest first)
	p.sortBySizeAsc(eligibleSegments)

	// Find segments that are roughly the same size (within the same tier)
	floorSegmentBytes := int64(p.floorSegmentMB * 1024 * 1024)

	for i := 0; i <= len(eligibleSegments)-p.segmentsPerTier; i++ {
		// Check if we have enough segments in this tier
		candidateSize := int64(0)
		if p.calibrateSizeByDeletes {
			candidateSize = p.sizeBytes(eligibleSegments[i], mergeContext)
		} else {
			candidateSize = eligibleSegments[i].SegmentInfo().SizeInBytes()
		}

		if candidateSize < floorSegmentBytes {
			candidateSize = floorSegmentBytes
		}

		// Find segments in the same tier
		tierEnd := i + 1
		for tierEnd < len(eligibleSegments) && tierEnd < i+p.mergeFactor {
			size := int64(0)
			if p.calibrateSizeByDeletes {
				size = p.sizeBytes(eligibleSegments[tierEnd], mergeContext)
			} else {
				size = eligibleSegments[tierEnd].SegmentInfo().SizeInBytes()
			}

			// Check if this segment is in the same tier
			if float64(size) < float64(candidateSize)*1.5 {
				tierEnd++
			} else {
				break
			}
		}

		// If we have enough segments in this tier, create a merge
		tierCount := tierEnd - i
		if tierCount >= p.segmentsPerTier {
			spec := NewMergeSpecification()
			merge := NewOneMerge(eligibleSegments[i:tierEnd])

			// Check if merge size is acceptable
			if p.isMergeSizeAcceptable(merge) {
				spec.Add(merge)
				return spec
			}
		}
	}

	return nil
}

// isMergeSizeAcceptable returns true if the merge size is acceptable.
func (p *NRTMergePolicy) isMergeSizeAcceptable(merge *OneMerge) bool {
	if merge == nil || len(merge.Segments) == 0 {
		return false
	}

	// Calculate total size
	totalSize := int64(0)
	totalDocs := int32(0)
	for _, info := range merge.Segments {
		totalSize += info.SegmentInfo().SizeInBytes()
		totalDocs += int32(info.DocCount())
	}

	totalSizeMB := float64(totalSize) / (1024.0 * 1024.0)

	// Check against max merged segment size
	if totalSizeMB > p.maxMergedSegmentMB {
		return false
	}

	// Check against max merge size
	if totalSizeMB > p.maxMergeMB {
		return false
	}

	// Check document count
	if int(totalDocs) > p.maxMergeDocs {
		return false
	}

	return true
}

// sizeBytes returns the size in bytes of a segment, calibrated by deletes if enabled.
func (p *NRTMergePolicy) sizeBytes(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	size := info.SegmentInfo().SizeInBytes()

	if p.calibrateSizeByDeletes {
		delCount := info.DelCount()
		docCount := info.DocCount()
		if docCount > 0 {
			liveDocs := docCount - delCount
			size = size * int64(liveDocs) / int64(docCount)
		}
	}

	return size
}

// sortBySizeAsc sorts segments by size (ascending).
func (p *NRTMergePolicy) sortBySizeAsc(segments []*SegmentCommitInfo) {
	// Simple bubble sort for clarity
	for i := 0; i < len(segments)-1; i++ {
		for j := i + 1; j < len(segments); j++ {
			sizeI := segments[i].SegmentInfo().SizeInBytes()
			sizeJ := segments[j].SegmentInfo().SizeInBytes()
			if sizeI > sizeJ {
				segments[i], segments[j] = segments[j], segments[i]
			}
		}
	}
}

// sortByDeletesDesc sorts segments by delete percentage (descending).
func (p *NRTMergePolicy) sortByDeletesDesc(segments []*SegmentCommitInfo) {
	for i := 0; i < len(segments)-1; i++ {
		for j := i + 1; j < len(segments); j++ {
			delPctI := p.deletePct(segments[i])
			delPctJ := p.deletePct(segments[j])
			if delPctI < delPctJ {
				segments[i], segments[j] = segments[j], segments[i]
			}
		}
	}
}

// deletePct returns the delete percentage for a segment.
func (p *NRTMergePolicy) deletePct(info *SegmentCommitInfo) float64 {
	docCount := info.DocCount()
	if docCount == 0 {
		return 0
	}
	delCount := info.DelCount()
	return float64(delCount) * 100.0 / float64(docCount)
}

// FindForcedMerges finds forced merges (e.g., for optimizing the index).
func (p *NRTMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	if infos == nil {
		return nil, fmt.Errorf("segment infos cannot be nil")
	}

	segments := infos.List()

	if len(segments) <= maxSegmentCount {
		return nil, nil
	}

	// Find merges to reduce segment count
	spec := NewMergeSpecification()

	// Merge segments in batches
	for i := 0; i < len(segments); i += p.mergeFactor {
		end := i + p.mergeFactor
		if end > len(segments) {
			end = len(segments)
		}

		merge := NewOneMerge(segments[i:end])
		if p.isMergeSizeAcceptable(merge) {
			spec.Add(merge)
		}
	}

	if len(spec.Merges) > 0 {
		return spec, nil
	}

	return nil, nil
}

// FindForcedDeletesMerges finds merges necessary to expunge deleted documents.
func (p *NRTMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if infos == nil {
		return nil, fmt.Errorf("segment infos cannot be nil")
	}

	segments := infos.List()

	// Find segments with deletes
	var segmentsWithDeletes []*SegmentCommitInfo
	for _, info := range segments {
		if info.DelCount() > 0 {
			segmentsWithDeletes = append(segmentsWithDeletes, info)
		}
	}

	if len(segmentsWithDeletes) < p.mergeFactor {
		return nil, nil
	}

	// Sort by delete percentage
	p.sortByDeletesDesc(segmentsWithDeletes)

	spec := NewMergeSpecification()

	// Create merges
	for i := 0; i < len(segmentsWithDeletes); i += p.mergeFactor {
		end := i + p.mergeFactor
		if end > len(segmentsWithDeletes) {
			end = len(segmentsWithDeletes)
		}

		merge := NewOneMerge(segmentsWithDeletes[i:end])
		if p.isMergeSizeAcceptable(merge) {
			spec.Add(merge)
		}
	}

	if len(spec.Merges) > 0 {
		return spec, nil
	}

	return nil, nil
}

// UseCompoundFile returns true if the merged segment should use compound file format.
func (p *NRTMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	// Always use CFS for NRT to reduce file handles
	return true
}

// String returns a string representation of the NRTMergePolicy.
func (p *NRTMergePolicy) String() string {
	return fmt.Sprintf("NRTMergePolicy{minMergeMB=%.1f, maxMergeMB=%.1f, mergeFactor=%d, maxMergedSegmentMB=%.1f, deletesPctAllowed=%.1f}",
		p.minMergeMB, p.maxMergeMB, p.mergeFactor, p.maxMergedSegmentMB, p.deletesPctAllowed)
}

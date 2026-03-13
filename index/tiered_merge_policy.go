// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"
)

// MergeType represents the type of merge being performed.
type MergeType int

const (
	// MergeTypeNatural is a normal merge triggered by the merge policy.
	MergeTypeNatural MergeType = iota
	// MergeTypeForceMerge is a forced merge (e.g., optimize).
	MergeTypeForceMerge
	// MergeTypeForceMergeDeletes is a forced merge to expunge deletes.
	MergeTypeForceMergeDeletes
)

// TieredMergePolicy merges segments of approximately equal size, subject to
// an allowed number of segments per tier.
// This is the Go port of Lucene's org.apache.lucene.index.TieredMergePolicy.
//
// This merge policy first computes a "budget" of how many segments are allowed
// to be in the index. If the index is over-budget, then the policy sorts segments
// by decreasing size (pro-rating by percent deletes), and then finds the
// least-cost merge. Merge cost is measured by a combination of the "skew" of
// the merge (size of largest segment divided by smallest segment), total merge
// size and percent deletes reclaimed.
type TieredMergePolicy struct {
	*BaseMergePolicy

	// maxMergedSegmentBytes is the maximum sized segment to produce during normal merging.
	// Default is 5 GB.
	maxMergedSegmentBytes int64

	// floorSegmentBytes is the minimum segment size to consider for merging.
	// Segments smaller than this are treated as this size.
	// Default is 16 MB.
	floorSegmentBytes int64

	// segsPerTier is the allowed number of segments per tier.
	// Smaller values mean more merging but fewer segments.
	// Default is 8.0.
	segsPerTier float64

	// forceMergeDeletesPctAllowed is the delete percentage threshold for forceMergeDeletes.
	// Default is 10.0.
	forceMergeDeletesPctAllowed float64

	// deletesPctAllowed is the maximum percentage of deleted documents allowed.
	// Default is 20.0.
	deletesPctAllowed float64

	// targetSearchConcurrency is the target search concurrency.
	// This prevents creating segments bigger than maxDoc/targetSearchConcurrency.
	// Default is 1.
	targetSearchConcurrency int

	// maxMergeAtOnce is the maximum number of segments to merge at once.
	// Default is 10.
	maxMergeAtOnce int

	// maxMergeAtOnceExplicit is the maximum number of segments to merge for explicit merges.
	// Default is 30.
	maxMergeAtOnceExplicit int

	// noCFSRatio is the ratio for using compound files.
	// If merged segment size / total index size < this ratio, use CFS.
	noCFSRatio float64
}

// NewTieredMergePolicy creates a new TieredMergePolicy with default settings.
func NewTieredMergePolicy() *TieredMergePolicy {
	return &TieredMergePolicy{
		BaseMergePolicy:             NewBaseMergePolicy(),
		maxMergedSegmentBytes:      5 * 1024 * 1024 * 1024, // 5GB
		floorSegmentBytes:          16 * 1024 * 1024,       // 16MB
		segsPerTier:                8.0,
		forceMergeDeletesPctAllowed: 10.0,
		deletesPctAllowed:          20.0,
		targetSearchConcurrency:    1,
		maxMergeAtOnce:             10,
		maxMergeAtOnceExplicit:     30,
		noCFSRatio:                 0.1,
	}
}

// GetMaxMergedSegmentMB returns the maximum merged segment size in MB.
func (p *TieredMergePolicy) GetMaxMergedSegmentMB() float64 {
	return float64(p.maxMergedSegmentBytes) / 1024.0 / 1024.0
}

// SetMaxMergedSegmentMB sets the maximum merged segment size in MB.
func (p *TieredMergePolicy) SetMaxMergedSegmentMB(v float64) {
	p.maxMergedSegmentBytes = int64(v * 1024 * 1024)
}

// GetFloorSegmentMB returns the floor segment size in MB.
func (p *TieredMergePolicy) GetFloorSegmentMB() float64 {
	return float64(p.floorSegmentBytes) / 1024.0 / 1024.0
}

// SetFloorSegmentMB sets the floor segment size in MB.
func (p *TieredMergePolicy) SetFloorSegmentMB(v float64) {
	if v <= 0.0 {
		v = 1.0
	}
	p.floorSegmentBytes = int64(v * 1024 * 1024)
}

// GetSegmentsPerTier returns the segments per tier.
func (p *TieredMergePolicy) GetSegmentsPerTier() float64 {
	return p.segsPerTier
}

// SetSegmentsPerTier sets the segments per tier.
func (p *TieredMergePolicy) SetSegmentsPerTier(v float64) {
	if v < 2.0 {
		v = 2.0
	}
	p.segsPerTier = v
}

// GetForceMergeDeletesPctAllowed returns the force merge deletes percentage allowed.
func (p *TieredMergePolicy) GetForceMergeDeletesPctAllowed() float64 {
	return p.forceMergeDeletesPctAllowed
}

// SetForceMergeDeletesPctAllowed sets the force merge deletes percentage allowed.
func (p *TieredMergePolicy) SetForceMergeDeletesPctAllowed(v float64) {
	if v < 0.0 {
		v = 0.0
	}
	if v > 100.0 {
		v = 100.0
	}
	p.forceMergeDeletesPctAllowed = v
}

// GetDeletesPctAllowed returns the deletes percentage allowed.
func (p *TieredMergePolicy) GetDeletesPctAllowed() float64 {
	return p.deletesPctAllowed
}

// SetDeletesPctAllowed sets the deletes percentage allowed.
func (p *TieredMergePolicy) SetDeletesPctAllowed(v float64) {
	if v <= 0 {
		v = 1.0
	}
	if v > 50.0 {
		v = 50.0
	}
	p.deletesPctAllowed = v
}

// GetTargetSearchConcurrency returns the target search concurrency.
func (p *TieredMergePolicy) GetTargetSearchConcurrency() int {
	return p.targetSearchConcurrency
}

// SetTargetSearchConcurrency sets the target search concurrency.
func (p *TieredMergePolicy) SetTargetSearchConcurrency(v int) {
	if v < 1 {
		v = 1
	}
	p.targetSearchConcurrency = v
}

// GetMaxMergeAtOnce returns the maximum number of segments to merge at once.
func (p *TieredMergePolicy) GetMaxMergeAtOnce() int {
	return p.maxMergeAtOnce
}

// SetMaxMergeAtOnce sets the maximum number of segments to merge at once.
func (p *TieredMergePolicy) SetMaxMergeAtOnce(v int) {
	if v < 2 {
		v = 2
	}
	p.maxMergeAtOnce = v
}

// GetMaxMergeAtOnceExplicit returns the maximum number of segments to merge at once for explicit merges.
func (p *TieredMergePolicy) GetMaxMergeAtOnceExplicit() int {
	return p.maxMergeAtOnceExplicit
}

// SetMaxMergeAtOnceExplicit sets the maximum number of segments to merge at once for explicit merges.
func (p *TieredMergePolicy) SetMaxMergeAtOnceExplicit(v int) {
	if v < 2 {
		v = 2
	}
	p.maxMergeAtOnceExplicit = v
}

// GetNoCFSRatio returns the compound file ratio.
func (p *TieredMergePolicy) GetNoCFSRatio() float64 {
	return p.noCFSRatio
}

// SetNoCFSRatio sets the compound file ratio.
func (p *TieredMergePolicy) SetNoCFSRatio(v float64) {
	p.noCFSRatio = v
}

// FindMerges finds merges based on tiered policy.
// This is the Go port of Lucene's TieredMergePolicy.findMerges().
func (p *TieredMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	merging := mergeContext.GetMergingSegments()

	// Compute total index bytes and segment details
	var totIndexBytes int64
	var minSegmentBytes int64 = math.MaxInt64
	var totalDelDocs, totalMaxDoc int
	var mergingBytes int64

	sortedInfos := p.getSortedBySegmentSize(infos, mergeContext)

	// Filter out merging segments and compute totals
	eligible := make([]*SegmentSizeAndDocs, 0, len(sortedInfos))
	for _, segSizeDocs := range sortedInfos {
		segBytes := segSizeDocs.SizeInBytes

		if merging[segSizeDocs.SegInfo] {
			mergingBytes += segSizeDocs.SizeInBytes
			totalMaxDoc += segSizeDocs.MaxDoc - segSizeDocs.DelCount
			continue
		}

		if segBytes >= p.maxMergedSegmentBytes {
			// Segment is too large, skip from merge consideration
			continue
		}

		eligible = append(eligible, segSizeDocs)
		totalDelDocs += segSizeDocs.DelCount
		totalMaxDoc += segSizeDocs.MaxDoc
		totIndexBytes += segBytes

		if segBytes < minSegmentBytes {
			minSegmentBytes = segBytes
		}
	}

	if len(eligible) == 0 {
		return nil, nil
	}

	// Calculate allowed segment count based on tier structure
	allowedSegCount := p.getAllowedSegmentCount(len(eligible), totIndexBytes, minSegmentBytes)
	allowedDelCount := int(p.deletesPctAllowed * float64(totalMaxDoc) / 100.0)

	// Calculate total delete percentage
	totalDelPct := 100.0 * float64(totalDelDocs) / float64(totalMaxDoc)

	// Remove too-large segments from consideration
	tooBigCount := 0
	finalEligible := make([]*SegmentSizeAndDocs, 0, len(eligible))
	for _, segSizeDocs := range eligible {
		segBytes := segSizeDocs.SizeInBytes
		segDelPct := 100.0 * float64(segSizeDocs.DelCount) / float64(segSizeDocs.MaxDoc)

		// Skip segments that are too large (unless they have high deletes)
		if segBytes > p.maxMergedSegmentBytes/2 && (totalDelPct <= p.deletesPctAllowed || segDelPct <= p.deletesPctAllowed) {
			tooBigCount++
			allowedDelCount -= segSizeDocs.DelCount
			continue
		}

		finalEligible = append(finalEligible, segSizeDocs)
	}

	if allowedDelCount < 0 {
		allowedDelCount = 0
	}

	// Ensure minimum allowed segment count
	allowedSegCount = maxInt(allowedSegCount, int(p.segsPerTier))
	allowedSegCount = maxInt(allowedSegCount, p.targetSearchConcurrency-tooBigCount)

	// Check if we need any merges
	remainingDelCount := 0
	for _, seg := range finalEligible {
		remainingDelCount += seg.DelCount
	}

	if len(finalEligible) <= allowedSegCount && remainingDelCount <= allowedDelCount {
		return nil, nil
	}

	// Find best merges
	return p.doFindMerges(finalEligible, p.maxMergedSegmentBytes, int(p.segsPerTier),
		allowedSegCount, allowedDelCount, MergeTypeNatural, mergeContext, mergingBytes >= p.maxMergedSegmentBytes)
}

// getSortedBySegmentSize returns segments sorted by size (largest first).
func (p *TieredMergePolicy) getSortedBySegmentSize(infos *SegmentInfos, mergeContext MergeContext) []*SegmentSizeAndDocs {
	sorted := make([]*SegmentSizeAndDocs, 0, infos.Size())

	for sci := range infos.Iterator() {
		size := p.Size(sci, mergeContext)
		delCount := mergeContext.NumDeletesToMerge(sci)
		sorted = append(sorted, NewSegmentSizeAndDocs(sci, size, delCount))
	}

	// Sort by largest size first
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SizeInBytes != sorted[j].SizeInBytes {
			return sorted[i].SizeInBytes > sorted[j].SizeInBytes
		}
		return sorted[i].Name < sorted[j].Name
	})

	return sorted
}

// getAllowedSegmentCount calculates the allowed segment count based on index size.
func (p *TieredMergePolicy) getAllowedSegmentCount(eligibleCount int, totIndexBytes, minSegmentBytes int64) int {
	mergeFactor := int(p.segsPerTier)

	// Calculate allowed segment count based on tier structure
	levelSize := maxInt64(minSegmentBytes, p.floorSegmentBytes)
	bytesLeft := totIndexBytes
	allowedSegCount := 0.0

	for {
		segCountLevel := float64(bytesLeft) / float64(levelSize)
		if segCountLevel < p.segsPerTier || levelSize == p.maxMergedSegmentBytes {
			allowedSegCount += math.Ceil(segCountLevel)
			break
		}
		allowedSegCount += p.segsPerTier
		bytesLeft -= int64(p.segsPerTier * float64(levelSize))
		levelSize = minInt64(p.maxMergedSegmentBytes, levelSize*int64(mergeFactor))
	}

	return int(allowedSegCount)
}

// doFindMerges finds the best merges from eligible segments.
func (p *TieredMergePolicy) doFindMerges(
	sortedEligible []*SegmentSizeAndDocs,
	maxMergedSegmentBytes int64,
	mergeFactor int,
	allowedSegCount int,
	allowedDelCount int,
	mergeType MergeType,
	mergeContext MergeContext,
	maxMergeIsRunning bool,
) (*MergeSpecification, error) {

	if len(sortedEligible) == 0 {
		return nil, nil
	}

	// Create size map for quick lookup
	segInfosSizes := make(map[*SegmentCommitInfo]*SegmentSizeAndDocs)
	for _, segSizeDocs := range sortedEligible {
		segInfosSizes[segSizeDocs.SegInfo] = segSizeDocs
	}

	var spec *MergeSpecification
	toBeMerged := make(map[*SegmentCommitInfo]bool)
	haveOneLargeMerge := false

	for {
		// Remove already merged segments from consideration
		eligible := make([]*SegmentSizeAndDocs, 0, len(sortedEligible))
		for _, seg := range sortedEligible {
			if !toBeMerged[seg.SegInfo] {
				eligible = append(eligible, seg)
			}
		}

		if len(eligible) == 0 {
			return spec, nil
		}

		// Check if we're under budget
		remainingDelCount := 0
		for _, seg := range eligible {
			remainingDelCount += seg.DelCount
		}

		if mergeType == MergeTypeNatural && len(eligible) <= allowedSegCount && remainingDelCount <= allowedDelCount {
			return spec, nil
		}

		// Find best merge
		var bestScore *MergeScore
		var bestMerge []*SegmentCommitInfo
		var bestTooLarge bool

		for startIdx := 0; startIdx < len(eligible); startIdx++ {
			candidate := make([]*SegmentCommitInfo, 0)
			var bytesThisMerge int64
			var docCountThisMerge int
			hitTooLarge := false

			for idx := startIdx; idx < len(eligible); idx++ {
				segSizeDocs := eligible[idx]
				segBytes := segSizeDocs.SizeInBytes
				segDocCount := segSizeDocs.MaxDoc - segSizeDocs.DelCount

				// Check merge limits
				if len(candidate) < mergeFactor || bytesThisMerge < p.floorSegmentBytes {
					if bytesThisMerge+segBytes > maxMergedSegmentBytes {
						hitTooLarge = true
						if len(candidate) > 0 {
							continue // Try to pack smaller segments
						}
					}

					if bytesThisMerge > p.floorSegmentBytes && docCountThisMerge+segDocCount > p.getMaxAllowedDocs() {
						if len(candidate) > 0 {
							continue
						}
					}

					candidate = append(candidate, segSizeDocs.SegInfo)
					bytesThisMerge += segBytes
					docCountThisMerge += segDocCount
				} else {
					break
				}
			}

			if len(candidate) == 0 {
				continue
			}

			// Score this merge candidate
			maxCandidateSeg := segInfosSizes[candidate[0]]
			if !hitTooLarge && mergeType == MergeTypeNatural &&
				bytesThisMerge < maxCandidateSeg.SizeInBytes*3/2 &&
				maxCandidateSeg.DelCount < maxCandidateSeg.MaxDoc*int(p.deletesPctAllowed)/100 {
				// Ignore merges where the result is not at least 50% larger
				// than the biggest input segment (avoid O(N^2) merging)
				continue
			}

			// Singleton merge with no deletes makes no sense
			if len(candidate) == 1 && maxCandidateSeg.DelCount == 0 {
				continue
			}

			// If we didn't find a too-large merge and have fewer than mergeFactor candidates, stop
			if bestScore != nil && !hitTooLarge && len(candidate) < mergeFactor {
				break
			}

			score := p.score(candidate, hitTooLarge, segInfosSizes)

			if (bestScore == nil || score.Score < bestScore.Score) && (!hitTooLarge || !maxMergeIsRunning) {
				bestMerge = candidate
				bestScore = score
				bestTooLarge = hitTooLarge
			}
		}

		if len(bestMerge) == 0 {
			return spec, nil
		}

		// Add merge to specification
		if !haveOneLargeMerge || !bestTooLarge || mergeType == MergeTypeForceMergeDeletes {
			haveOneLargeMerge = haveOneLargeMerge || bestTooLarge

			if spec == nil {
				spec = NewMergeSpecification()
			}

			merge := NewOneMerge(bestMerge)
			spec.Add(merge)

			for _, seg := range bestMerge {
				toBeMerged[seg] = true
			}
		}

		// Continue loop to find more merges
	}
}

// MergeScore holds the score and explanation for a merge candidate.
type MergeScore struct {
	// Score is the merge score (lower is better).
	Score float64

	// Skew is the skew factor of the merge.
	Skew float64

	// NonDelRatio is the ratio of non-deleted documents.
	NonDelRatio float64
}

// score calculates the score for a merge candidate.
// Lower scores are better.
func (p *TieredMergePolicy) score(
	candidate []*SegmentCommitInfo,
	hitTooLarge bool,
	segInfosSizes map[*SegmentCommitInfo]*SegmentSizeAndDocs,
) *MergeScore {
	var totBeforeMergeBytes, totAfterMergeBytes, totAfterMergeBytesFloored int64

	for _, info := range candidate {
		segBytes := segInfosSizes[info].SizeInBytes
		totAfterMergeBytes += segBytes
		totAfterMergeBytesFloored += p.floorSize(segBytes)
		totBeforeMergeBytes += info.SegmentInfo().SizeInBytes()
	}

	// Calculate skew (measure of merge balance)
	// Skew ranges from 1.0/numSegsBeingMerged (good) to 1.0 (poor)
	var skew float64
	if hitTooLarge {
		// Pretend perfect skew for too-large merges
		mergeFactor := int(p.segsPerTier)
		skew = 1.0 / float64(mergeFactor)
	} else {
		largestSize := float64(p.floorSize(segInfosSizes[candidate[0]].SizeInBytes))
		skew = largestSize / float64(totAfterMergeBytesFloored)
	}

	// Merge score: strongly favor less skew, gently favor smaller merges
	mergeScore := skew
	mergeScore *= math.Pow(float64(totAfterMergeBytes), 0.05)

	// Strongly favor merges that reclaim deletes
	nonDelRatio := float64(totAfterMergeBytes) / float64(totBeforeMergeBytes)
	mergeScore *= math.Pow(nonDelRatio, 2)

	return &MergeScore{
		Score:       mergeScore,
		Skew:        skew,
		NonDelRatio: nonDelRatio,
	}
}

// Score calculates and returns the merge score for a candidate merge.
// This is an exported wrapper for testing.
// Lower scores are better.
func (p *TieredMergePolicy) Score(
	candidate []*SegmentCommitInfo,
	hitTooLarge bool,
	segInfosSizes map[*SegmentCommitInfo]*SegmentSizeAndDocs,
) float64 {
	result := p.score(candidate, hitTooLarge, segInfosSizes)
	return result.Score
}

// floorSize returns the size floored to floorSegmentBytes.
func (p *TieredMergePolicy) floorSize(bytes int64) int64 {
	if bytes < p.floorSegmentBytes {
		return p.floorSegmentBytes
	}
	return bytes
}

// getMaxAllowedDocs calculates the maximum allowed documents per segment for concurrency.
func (p *TieredMergePolicy) getMaxAllowedDocs() int {
	// Simplified implementation - in Lucene this considers totalMaxDoc and totalDelDocs
	return math.MaxInt32
}

// FindForcedMerges finds forced merges to reduce segment count.
func (p *TieredMergePolicy) FindForcedMerges(
	infos *SegmentInfos,
	maxSegmentCount int,
	segmentsToMerge map[*SegmentCommitInfo]bool,
	mergeContext MergeContext,
) (*MergeSpecification, error) {

	sortedInfos := p.getSortedBySegmentSize(infos, mergeContext)
	merging := mergeContext.GetMergingSegments()

	var totalMergeBytes int64

	// Filter segments
	eligible := make([]*SegmentSizeAndDocs, 0)
	forceMergeRunning := false

	for _, segSizeDocs := range sortedInfos {
		_, shouldMerge := segmentsToMerge[segSizeDocs.SegInfo]
		if !shouldMerge {
			continue
		}

		if merging[segSizeDocs.SegInfo] {
			forceMergeRunning = true
			continue
		}

		totalMergeBytes += segSizeDocs.SizeInBytes
		eligible = append(eligible, segSizeDocs)
	}

	// Calculate max merge size based on target segment count
	maxMergeBytes := p.maxMergedSegmentBytes
	if maxSegmentCount == 1 {
		maxMergeBytes = math.MaxInt64
	} else if maxSegmentCount != math.MaxInt32 {
		// Estimate based on total size and target count
		maxMergeBytes = maxInt64(int64(float64(totalMergeBytes)/float64(maxSegmentCount)), p.maxMergedSegmentBytes)
		// Fudge up 25% to avoid needing a second pass
		maxMergeBytes = int64(float64(maxMergeBytes) * 1.25)
	}

	// Filter out segments that are too big with no deletes
	finalEligible := make([]*SegmentSizeAndDocs, 0)
	for _, seg := range eligible {
		if maxSegmentCount != math.MaxInt32 && seg.SizeInBytes >= maxMergeBytes && seg.DelCount == 0 {
			continue
		}
		finalEligible = append(finalEligible, seg)
	}

	if len(finalEligible) == 0 {
		return nil, nil
	}

	// Check if we're already at target
	if maxSegmentCount != math.MaxInt32 && maxSegmentCount > 1 && len(finalEligible) <= maxSegmentCount {
		return nil, nil
	}

	// Special case: merge to one segment
	if maxSegmentCount == 1 && totalMergeBytes < maxMergeBytes {
		allSegments := make([]*SegmentCommitInfo, len(finalEligible))
		for i, seg := range finalEligible {
			allSegments[i] = seg.SegInfo
		}
		spec := NewMergeSpecification()
		spec.Add(NewOneMerge(allSegments))
		return spec, nil
	}

	if forceMergeRunning {
		return nil, nil
	}

	// Bin-packing: merge from largest to smallest
	var spec *MergeSpecification
	index := len(finalEligible) - 1
	resultingSegments := len(finalEligible)

	for {
		candidate := make([]*SegmentCommitInfo, 0)
		var currentCandidateBytes int64

		for index >= 0 && resultingSegments > maxSegmentCount {
			current := finalEligible[index].SegInfo
			initialCandidateSize := len(candidate)
			currentSegmentSize := current.SegmentInfo().SizeInBytes()

			if currentCandidateBytes+currentSegmentSize <= maxMergeBytes || initialCandidateSize < 2 {
				candidate = append(candidate, current)
				index--
				currentCandidateBytes += currentSegmentSize
				if initialCandidateSize > 0 {
					resultingSegments--
				}
			} else {
				break
			}
		}

		if len(candidate) > 1 {
			if spec == nil {
				spec = NewMergeSpecification()
			}
			spec.Add(NewOneMerge(candidate))
		} else {
			break
		}
	}

	return spec, nil
}

// FindForcedDeletesMerges finds merges to expunge deleted documents.
func (p *TieredMergePolicy) FindForcedDeletesMerges(
	infos *SegmentInfos,
	mergeContext MergeContext,
) (*MergeSpecification, error) {

	merging := mergeContext.GetMergingSegments()

	// First check if there's any work to do
	haveWork := false
	for sci := range infos.Iterator() {
		delCount := mergeContext.NumDeletesToMerge(sci)
		pctDeletes := 100.0 * float64(delCount) / float64(sci.SegmentInfo().DocCount())
		if pctDeletes > p.forceMergeDeletesPctAllowed && !merging[sci] {
			haveWork = true
			break
		}
	}

	if !haveWork {
		return nil, nil
	}

	sortedInfos := p.getSortedBySegmentSize(infos, mergeContext)

	// Filter eligible segments
	eligible := make([]*SegmentSizeAndDocs, 0)
	for _, segSizeDocs := range sortedInfos {
		pctDeletes := 100.0 * float64(segSizeDocs.DelCount) / float64(segSizeDocs.MaxDoc)
		if !merging[segSizeDocs.SegInfo] && pctDeletes > p.forceMergeDeletesPctAllowed {
			eligible = append(eligible, segSizeDocs)
		}
	}

	if len(eligible) == 0 {
		return nil, nil
	}

	// Use relaxed constraints for forced deletes
	return p.doFindMerges(eligible, p.maxMergedSegmentBytes, math.MaxInt,
		math.MaxInt, 0, MergeTypeForceMergeDeletes, mergeContext, false)
}

// UseCompoundFile returns true if the merged segment should use compound file.
func (p *TieredMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	if p.noCFSRatio >= 1.0 {
		return false
	}
	if p.noCFSRatio <= 0.0 {
		return true
	}

	// Calculate total index size
	var totalSize int64
	for sci := range infos.Iterator() {
		totalSize += sci.SegmentInfo().SizeInBytes()
	}

	if totalSize == 0 {
		return true
	}

	mergedSize := mergedSegmentInfo.SizeInBytes()
	ratio := float64(mergedSize) / float64(totalSize)

	return ratio < p.noCFSRatio
}

// NumDeletesToMerge returns the number of deletes for a segment.
func (p *TieredMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return delCount
}

// String returns a string representation of the policy.
func (p *TieredMergePolicy) String() string {
	return fmt.Sprintf("[TieredMergePolicy: maxMergedSegmentMB=%.1f, floorSegmentMB=%.1f, "+
		"forceMergeDeletesPctAllowed=%.1f, segmentsPerTier=%.1f, deletesPctAllowed=%.1f]",
		p.GetMaxMergedSegmentMB(), p.GetFloorSegmentMB(), p.forceMergeDeletesPctAllowed,
		p.segsPerTier, p.deletesPctAllowed)
}

// maxInt returns the maximum of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// maxInt64 returns the maximum of two int64s.
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// minInt64 returns the minimum of two int64s.
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
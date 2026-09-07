// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// TieredMergePolicy merges segments of approximately equal size, subject to an allowed number of segments per tier.
// This is similar to LogByteSizeMergePolicy, except this merge policy is able to merge
// non-adjacent segments, and separates how many segments are merged at once (SetMaxMergeAtOnce)
// from how many segments are allowed per tier (SetSegmentsPerTier).
// This merge policy also does not over-merge (i.e. cascade merges).
//
// For normal merging, this policy first computes a "budget" of how many segments are allowed to
// be in the index. If the index is over-budget, then the policy sorts segments by decreasing size
// (pro-rating by percent deletes), and then finds the least-cost merge. Merge cost is measured by a
// combination of the "skew" of the merge (size of largest segment divided by smallest segment),
// total merge size and percent deletes reclaimed, so that merges with lower skew, smaller size and
// those reclaiming more deletes, are favored.
//
// If a merge will produce a segment that's larger than SetMaxMergedSegmentMB, then the
// policy will merge fewer segments (down to 1 at once, if that one has deletions) to keep the
// segment size under budget.
//
// NOTE: this policy freely merges non-adjacent segments; if this is a problem, use LogMergePolicy.
//
// NOTE: This policy always merges by byte size of the segments, always pro-rates by
// percent deletes.
//
// NOTE Starting with Lucene 7.5, if you call IndexWriter.ForceMerge(int) with
// this (default) merge policy, if SetMaxMergedSegmentMB is in conflict with
// maxNumSegments passed to IndexWriter.ForceMerge then maxNumSegments wins. For
// example, if your index has 50 1 GB segments, and you have SetMaxMergedSegmentMB at 1024
// (1 GB), and you call ForceMerge(10), the two settings are clearly in conflict. TieredMergePolicy
// will choose to break the SetMaxMergedSegmentMB constraint and try to
// merge down to at most ten segments, each up to 5 * 1.25 GB in size (since an extra 25% buffer
// increase in the expected segment size is targetted).
//
// findForcedDeletesMerges should never produce segments greater than maxSegmentSize.
//
// NOTE: This policy returns natural merges whose size is below the SetFloorSegmentMB(double) floor segment size
// for FindFullFlushMerges full-flush merges.
type TieredMergePolicy struct {
	*BaseMergePolicy

	maxMergeAtOnce              int
	maxMergedSegmentBytes       int64
	floorSegmentBytes           int64
	segsPerTier                 float64
	forceMergeDeletesPctAllowed float64
	deletesPctAllowed           float64
	targetSearchConcurrency     int
}

const defaultNoCFSRatio = 0.1

// NewTieredMergePolicy creates a TieredMergePolicy with all settings to their defaults.
func NewTieredMergePolicy() *TieredMergePolicy {
	return &TieredMergePolicy{
		BaseMergePolicy:             NewBaseMergePolicyWithDefaults(defaultNoCFSRatio, DefaultMaxCFSSegmentSize),
		maxMergeAtOnce:              10,
		maxMergedSegmentBytes:       5 * 1024 * 1024 * 1024,
		floorSegmentBytes:           16 * 1024 * 1024,
		segsPerTier:                 8.0,
		forceMergeDeletesPctAllowed: 10.0,
		deletesPctAllowed:           20.0,
		targetSearchConcurrency:     1,
	}
}

// SetMaxMergeAtOnce sets the maximum number of segments to be merged at a time during "normal" merging.
// Default is 10.
func (p *TieredMergePolicy) SetMaxMergeAtOnce(v int) *TieredMergePolicy {
	if v < 2 {
		panic(fmt.Sprintf("maxMergeAtOnce must be > 1 (got %d)", v))
	}
	p.maxMergeAtOnce = v
	return p
}

func (p *TieredMergePolicy) GetMaxMergeAtOnce() int {
	return p.maxMergeAtOnce
}

type mergeType int

const (
	mergeTypeNatural mergeType = iota
	mergeTypeForceMerge
	mergeTypeForceMergeDeletes
)

// SetMaxMergedSegmentMB sets the maximum sized segment to produce during normal merging.
// This setting is approximate: the estimate of the merged segment size is made by summing sizes of
// to-be-merged segments (compensating for percent deleted docs). Default is 5 GB.
func (p *TieredMergePolicy) SetMaxMergedSegmentMB(v float64) *TieredMergePolicy {
	if v < 0.0 {
		panic(fmt.Sprintf("maxMergedSegmentMB must be >=0 (got %f)", v))
	}
	vBytes := v * 1024 * 1024
	if vBytes > math.MaxInt64 {
		p.maxMergedSegmentBytes = math.MaxInt64
	} else {
		p.maxMergedSegmentBytes = int64(vBytes)
	}
	return p
}

// GetMaxMergedSegmentMB returns the current maxMergedSegmentMB setting.
func (p *TieredMergePolicy) GetMaxMergedSegmentMB() float64 {
	return float64(p.maxMergedSegmentBytes) / 1024.0 / 1024.0
}

// SetDeletesPctAllowed sets the maximum percentage of doc id space taken by deleted docs.
// Values must be between 0 and 50. Default value is 20.
func (p *TieredMergePolicy) SetDeletesPctAllowed(v float64) *TieredMergePolicy {
	if v <= 0 || v > 50 {
		panic(fmt.Sprintf("indexPctDeletedTarget must be > 0 and <= 50 (got %f)", v))
	}
	p.deletesPctAllowed = v
	return p
}

// GetDeletesPctAllowed returns the current deletesPctAllowed setting.
func (p *TieredMergePolicy) GetDeletesPctAllowed() float64 {
	return p.deletesPctAllowed
}

// SetFloorSegmentMB sets the floor segment size. Segments smaller than this size are merged
// more aggressively. Default is 16MB.
func (p *TieredMergePolicy) SetFloorSegmentMB(v float64) *TieredMergePolicy {
	if v <= 0.0 {
		panic(fmt.Sprintf("floorSegmentMB must be > 0.0 (got %f)", v))
	}
	vBytes := v * 1024 * 1024
	if vBytes > math.MaxInt64 {
		p.floorSegmentBytes = math.MaxInt64
	} else {
		p.floorSegmentBytes = int64(vBytes)
	}
	return p
}

// GetFloorSegmentMB returns the current floorSegmentMB.
func (p *TieredMergePolicy) GetFloorSegmentMB() float64 {
	return float64(p.floorSegmentBytes) / (1024 * 1024.0)
}

// MaxFullFlushMergeSize returns the maximum size of segments to include in full-flush merges.
func (p *TieredMergePolicy) MaxFullFlushMergeSize() int64 {
	return p.floorSegmentBytes
}

// SetForceMergeDeletesPctAllowed sets the threshold above which a segment is merged
// when forceMergeDeletes is called. Default is 10%.
func (p *TieredMergePolicy) SetForceMergeDeletesPctAllowed(v float64) *TieredMergePolicy {
	if v < 0.0 || v > 100.0 {
		panic(fmt.Sprintf("forceMergeDeletesPctAllowed must be between 0.0 and 100.0 inclusive (got %f)", v))
	}
	p.forceMergeDeletesPctAllowed = v
	return p
}

// GetForceMergeDeletesPctAllowed returns the current forceMergeDeletesPctAllowed setting.
func (p *TieredMergePolicy) GetForceMergeDeletesPctAllowed() float64 {
	return p.forceMergeDeletesPctAllowed
}

// SetSegmentsPerTier sets the allowed number of segments per tier. Default is 8.0.
func (p *TieredMergePolicy) SetSegmentsPerTier(v float64) *TieredMergePolicy {
	if v < 2.0 {
		panic(fmt.Sprintf("segmentsPerTier must be >= 2.0 (got %f)", v))
	}
	p.segsPerTier = v
	return p
}

// GetSegmentsPerTier returns the current segmentsPerTier setting.
func (p *TieredMergePolicy) GetSegmentsPerTier() float64 {
	return p.segsPerTier
}

// SetTargetSearchConcurrency sets the target search concurrency.
func (p *TieredMergePolicy) SetTargetSearchConcurrency(v int) *TieredMergePolicy {
	if v < 1 {
		panic(fmt.Sprintf("targetSearchConcurrency must be >= 1 (got %d)", v))
	}
	p.targetSearchConcurrency = v
	return p
}

// GetTargetSearchConcurrency returns the target search concurrency.
func (p *TieredMergePolicy) GetTargetSearchConcurrency() int {
	return p.targetSearchConcurrency
}

type segmentSizeAndDocs struct {
	segInfo     *SegmentCommitInfo
	sizeInBytes int64
	delCount    int
	maxDoc      int
	name        string
}

type mergeScore interface {
	GetScore() float64
	GetExplanation() string
}

type simpleMergeScore struct {
	score       float64
	explanation string
}

func (s simpleMergeScore) GetScore() float64      { return s.score }
func (s simpleMergeScore) GetExplanation() string { return s.explanation }

func (p *TieredMergePolicy) getSortedBySegmentSize(infos *SegmentInfos, mergeContext MergeContext) ([]*segmentSizeAndDocs, error) {
	sortedBySize := make([]*segmentSizeAndDocs, 0, infos.Size())

	for info := range infos.Iterator() {
		size, err := p.Size(info, mergeContext)
		if err != nil {
			return nil, err
		}
		sortedBySize = append(sortedBySize, &segmentSizeAndDocs{
			segInfo:     info,
			sizeInBytes: size,
			delCount:    mergeContext.NumDeletesToMerge(info),
			maxDoc:      info.MaxDoc(),
			name:        info.SegmentInfo().Name(),
		})
	}

	slices.SortFunc(sortedBySize, func(a, b *segmentSizeAndDocs) int {
		if a.sizeInBytes != b.sizeInBytes {
			if a.sizeInBytes > b.sizeInBytes {
				return -1
			}
			return 1
		}
		if a.name < b.name {
			return -1
		}
		if a.name > b.name {
			return 1
		}
		return 0
	})

	return sortedBySize, nil
}

func (p *TieredMergePolicy) FindMerges(mergeTrigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	merging := mergeContext.GetMergingSegments()
	var totIndexBytes int64
	var minSegmentBytes int64 = math.MaxInt64

	var totalDelDocs int
	var totalMaxDoc int
	var mergingBytes int64

	sortedInfos, err := p.getSortedBySegmentSize(infos, mergeContext)
	if err != nil {
		return nil, err
	}

	eligible := make([]*segmentSizeAndDocs, 0, len(sortedInfos))
	for _, segSizeDocs := range sortedInfos {
		segBytes := segSizeDocs.sizeInBytes
		if p.Verbose(mergeContext) {
			extra := ""
			if merging[segSizeDocs.segInfo] {
				extra = " [merging]"
			}
			if segBytes >= p.maxMergedSegmentBytes {
				extra += " [skip: too large]"
			} else if segBytes < p.floorSegmentBytes {
				extra += " [floored]"
			}
			p.Message(fmt.Sprintf("  seg=%s size=%.3f MB%s", p.SegString(mergeContext, []*SegmentCommitInfo{segSizeDocs.segInfo}), float64(segBytes)/1024.0/1024.0, extra), mergeContext)
		}

		if merging[segSizeDocs.segInfo] {
			mergingBytes += segSizeDocs.sizeInBytes
			totalMaxDoc += segSizeDocs.maxDoc - segSizeDocs.delCount
		} else {
			totalDelDocs += segSizeDocs.delCount
			totalMaxDoc += segSizeDocs.maxDoc
			eligible = append(eligible, segSizeDocs)
		}

		if segBytes < minSegmentBytes {
			minSegmentBytes = segBytes
		}
		totIndexBytes += segBytes
	}

	totalDelPct := 100.0 * float64(totalDelDocs) / float64(totalMaxDoc)
	allowedDelCount := int(p.deletesPctAllowed * float64(totalMaxDoc) / 100.0)

	tooBigCount := 0
	concurrencyCount := 0
	var allowedSegCount float64

	filteredEligible := make([]*segmentSizeAndDocs, 0, len(eligible))
	for _, segSizeDocs := range eligible {
		segDelPct := 100.0 * float64(segSizeDocs.delCount) / float64(segSizeDocs.maxDoc)
		if segSizeDocs.sizeInBytes > p.maxMergedSegmentBytes/2 && (totalDelPct <= p.deletesPctAllowed || segDelPct <= p.deletesPctAllowed) {
			tooBigCount++
			totIndexBytes -= segSizeDocs.sizeInBytes
			allowedDelCount -= segSizeDocs.delCount
		} else if concurrencyCount+tooBigCount < p.targetSearchConcurrency-1 {
			concurrencyCount++
			allowedSegCount++
			totIndexBytes -= segSizeDocs.sizeInBytes
		} else {
			filteredEligible = append(filteredEligible, segSizeDocs)
		}
	}
	if allowedDelCount < 0 {
		allowedDelCount = 0
	}

	mergeFactor := p.maxMergeAtOnce
	if int(p.segsPerTier) < mergeFactor {
		mergeFactor = int(p.segsPerTier)
	}

	levelSize := minSegmentBytes
	if p.floorSegmentBytes > levelSize {
		levelSize = p.floorSegmentBytes
	}
	bytesLeft := totIndexBytes
	for {
		segCountLevel := float64(bytesLeft) / float64(levelSize)
		if segCountLevel < p.segsPerTier || levelSize == p.maxMergedSegmentBytes {
			allowedSegCount += math.Ceil(segCountLevel)
			break
		}
		allowedSegCount += p.segsPerTier
		bytesLeft -= int64(p.segsPerTier) * levelSize
		newLevelSize := levelSize * int64(mergeFactor)
		if newLevelSize > p.maxMergedSegmentBytes {
			levelSize = p.maxMergedSegmentBytes
		} else {
			levelSize = newLevelSize
		}
	}

	if allowedSegCount < p.segsPerTier {
		allowedSegCount = p.segsPerTier
	}
	if allowedSegCount < float64(p.targetSearchConcurrency-tooBigCount) {
		allowedSegCount = float64(p.targetSearchConcurrency - tooBigCount)
	}
	allowedDocCount := p.getMaxAllowedDocs(totalMaxDoc, totalDelDocs)

	if p.Verbose(mergeContext) && tooBigCount > 0 {
		p.Message(fmt.Sprintf("  allowedSegmentCount=%.0f vs count=%d (eligible count=%d) tooBigCount= %d  allowedDocCount=%d vs doc count=%d", allowedSegCount, infos.Size(), len(filteredEligible), tooBigCount, allowedDocCount, infos.TotalMaxDoc()), mergeContext)
	}

	return p.doFindMerges(
		filteredEligible,
		p.maxMergedSegmentBytes,
		mergeFactor,
		int(allowedSegCount),
		allowedDelCount,
		allowedDocCount,
		mergeTypeNatural,
		mergeContext,
		mergingBytes >= p.maxMergedSegmentBytes,
	)
}

func (p *TieredMergePolicy) doFindMerges(
	sortedEligibleInfos []*segmentSizeAndDocs,
	maxMergedSegmentBytes int64,
	mergeFactor int,
	allowedSegCount int,
	allowedDelCount int,
	allowedDocCount int,
	mergeType mergeType,
	mergeContext MergeContext,
	maxMergeIsRunning bool,
) (*MergeSpecification, error) {
	sortedEligible := append([]*segmentSizeAndDocs(nil), sortedEligibleInfos...)

	segInfosSizes := make(map[*SegmentCommitInfo]*segmentSizeAndDocs)
	for _, segSizeDocs := range sortedEligible {
		segInfosSizes[segSizeDocs.segInfo] = segSizeDocs
	}

	originalSortedSize := len(sortedEligible)
	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("findMerges: %d segments", originalSortedSize), mergeContext)
	}
	if originalSortedSize == 0 {
		return nil, nil
	}

	toBeMerged := make(map[*SegmentCommitInfo]bool)
	var spec *MergeSpecification
	haveOneLargeMerge := false

	for {
		eligible := make([]*segmentSizeAndDocs, 0, len(sortedEligible))
		for _, segSizeDocs := range sortedEligible {
			if !toBeMerged[segSizeDocs.segInfo] {
				eligible = append(eligible, segSizeDocs)
			}
		}

		if p.Verbose(mergeContext) {
			p.Message(fmt.Sprintf("  allowedSegmentCount=%d vs count=%d (eligible count=%d)", allowedSegCount, originalSortedSize, len(eligible)), mergeContext)
		}

		if len(eligible) == 0 {
			return spec, nil
		}

		remainingDelCount := 0
		for _, c := range eligible {
			remainingDelCount += c.delCount
		}

		if mergeType == mergeTypeNatural && len(eligible) <= allowedSegCount && remainingDelCount <= allowedDelCount {
			return spec, nil
		}

		var bestScore mergeScore
		var best []*SegmentCommitInfo
		var bestTooLarge bool
		var bestMergeBytes int64

		for startIdx := 0; startIdx < len(eligible); startIdx++ {
			candidate := make([]*SegmentCommitInfo, 0, p.maxMergeAtOnce)
			hitTooLarge := false
			var bytesThisMerge int64
			var docCountThisMerge int

			for idx := startIdx; idx < len(eligible) && len(candidate) < p.maxMergeAtOnce &&
				(len(candidate) < mergeFactor || bytesThisMerge < p.floorSegmentBytes) &&
				bytesThisMerge < maxMergedSegmentBytes &&
				(bytesThisMerge < p.floorSegmentBytes || docCountThisMerge <= allowedDocCount); idx++ {

				segSizeDocs := eligible[idx]
				segBytes := segSizeDocs.sizeInBytes
				segDocCount := segSizeDocs.maxDoc - segSizeDocs.delCount

				if bytesThisMerge+segBytes > maxMergedSegmentBytes || (bytesThisMerge > p.floorSegmentBytes && docCountThisMerge+segDocCount > allowedDocCount) {
					if bytesThisMerge+segBytes > maxMergedSegmentBytes {
						hitTooLarge = true
					}
					if len(candidate) > 0 {
						continue
					}
				}
				candidate = append(candidate, segSizeDocs.segInfo)
				bytesThisMerge += segBytes
				docCountThisMerge += segDocCount
			}

			if len(candidate) == 0 {
				continue
			}

			maxCandidateSegmentSize := segInfosSizes[candidate[0]]
			if !hitTooLarge && mergeType == mergeTypeNatural &&
				bytesThisMerge < int64(float64(maxCandidateSegmentSize.sizeInBytes)*1.5) &&
				float64(maxCandidateSegmentSize.delCount) < float64(maxCandidateSegmentSize.maxDoc)*p.deletesPctAllowed/100.0 {
				continue
			}

			if len(candidate) == 1 && maxCandidateSegmentSize.delCount == 0 {
				continue
			}

			if bestScore != nil && !hitTooLarge && len(candidate) < mergeFactor {
				break
			}

			score := p.score(candidate, hitTooLarge, segInfosSizes)
			if p.Verbose(mergeContext) {
				p.Message(fmt.Sprintf("  maybe=%s score=%.3f %s tooLarge=%t size=%.3f MB", p.SegString(mergeContext, candidate), score.GetScore(), score.GetExplanation(), hitTooLarge, float64(bytesThisMerge)/1024.0/1024.0), mergeContext)
			}

			if (bestScore == nil || score.GetScore() < bestScore.GetScore()) && (!hitTooLarge || !maxMergeIsRunning) {
				best = candidate
				bestScore = score
				bestTooLarge = hitTooLarge
				bestMergeBytes = bytesThisMerge
			}
		}

		if best == nil {
			return spec, nil
		}

		if !haveOneLargeMerge || !bestTooLarge || mergeType == mergeTypeForceMergeDeletes {
			if bestTooLarge {
				haveOneLargeMerge = true
			}
			if spec == nil {
				spec = NewMergeSpecification()
			}
			spec.Add(NewOneMerge(best))

			if p.Verbose(mergeContext) {
				p.Message(fmt.Sprintf("  add merge=%s size=%.3f MB score=%.3f %s %s", p.SegString(mergeContext, best), float64(bestMergeBytes)/1024.0/1024.0, bestScore.GetScore(), bestScore.GetExplanation(), func() string {
					if bestTooLarge {
						return " [max merge]"
					}
					return ""
				}()), mergeContext)
			}
		}
		for _, b := range best {
			toBeMerged[b] = true
		}
	}
}

func (p *TieredMergePolicy) score(candidate []*SegmentCommitInfo, hitTooLarge bool, segmentsSizes map[*SegmentCommitInfo]*segmentSizeAndDocs) mergeScore {
	var totBeforeMergeBytes int64
	var totAfterMergeBytes int64
	var totAfterMergeBytesFloored int64

	for _, info := range candidate {
		segBytes := segmentsSizes[info].sizeInBytes
		totAfterMergeBytes += segBytes
		totAfterMergeBytesFloored += p.floorSize(segBytes)
		totBeforeMergeBytes += info.SizeInBytes() // Simplified, should use p.Size if available
	}

	var skew float64
	if hitTooLarge {
		mergeFactor := p.maxMergeAtOnce
		if int(p.segsPerTier) < mergeFactor {
			mergeFactor = int(p.segsPerTier)
		}
		skew = 1.0 / float64(mergeFactor)
	} else {
		skew = float64(p.floorSize(segmentsSizes[candidate[0]].sizeInBytes)) / float64(totAfterMergeBytesFloored)
	}

	mergeScore := skew
	mergeScore *= math.Pow(float64(totAfterMergeBytes), 0.05)
	nonDelRatio := float64(totAfterMergeBytes) / float64(totBeforeMergeBytes)
	mergeScore *= math.Pow(nonDelRatio, 2)

	return simpleMergeScore{
		score:       mergeScore,
		explanation: fmt.Sprintf("skew=%.3f nonDelRatio=%.3f", skew, nonDelRatio),
	}
}

func (p *TieredMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("findForcedMerges maxSegmentCount=%d infos=%s segmentsToMerge=%v", maxSegmentCount, p.SegString(mergeContext, infos.IteratorToSlice()), segmentsToMerge), mergeContext)
	}

	sortedSizeAndDocs, err := p.getSortedBySegmentSize(infos, mergeContext)
	if err != nil {
		return nil, err
	}

	var totalMergeBytes int64
	merging := mergeContext.GetMergingSegments()

	eligible := make([]*segmentSizeAndDocs, 0, len(sortedSizeAndDocs))
	forceMergeRunning := false
	for _, segSizeDocs := range sortedSizeAndDocs {
		isOriginal, ok := segmentsToMerge[segSizeDocs.segInfo]
		if !ok {
			continue
		}
		if merging[segSizeDocs.segInfo] {
			forceMergeRunning = true
			continue
		}
		totalMergeBytes += segSizeDocs.sizeInBytes
		eligible = append(eligible, segSizeDocs)
	}

	maxMergeBytes := p.maxMergedSegmentBytes

	if maxSegmentCount == 1 {
		maxMergeBytes = math.MaxInt64
	} else if maxSegmentCount != math.MaxInt32 {
		m := float64(totalMergeBytes) / float64(maxSegmentCount)
		if m > float64(p.maxMergedSegmentBytes) {
			m = float64(p.maxMergedSegmentBytes)
		}
		maxMergeBytes = int64(m * 1.25)
	}

	filteredEligible := make([]*segmentSizeAndDocs, 0, len(eligible))
	foundDeletes := false
	for _, segSizeDocs := range eligible {
		if segSizeDocs.delCount != 0 {
			if isOriginal, ok := segmentsToMerge[segSizeDocs.segInfo]; ok && isOriginal {
				foundDeletes = true
			}
			continue
		}
		if maxSegmentCount == math.MaxInt32 {
			if isOriginal, ok := segmentsToMerge[segSizeDocs.segInfo]; ok && !isOriginal {
				continue
			}
		}
		if maxSegmentCount != math.MaxInt32 && segSizeDocs.sizeInBytes >= maxMergeBytes {
			continue
		}
		filteredEligible = append(filteredEligible, segSizeDocs)
	}

	if len(filteredEligible) == 0 {
		return nil, nil
	}

	if !foundDeletes {
		infoZero := sortedSizeAndDocs[0].segInfo
		if (maxSegmentCount != math.MaxInt32 && maxSegmentCount > 1 && len(filteredEligible) <= maxSegmentCount) ||
			(maxSegmentCount == 1 && len(filteredEligible) == 1 && (segmentsToMerge[infoZero] || p.IsMerged(infos, infoZero, mergeContext))) {
			if p.Verbose(mergeContext) {
				p.Message("already merged", mergeContext)
			}
			return nil, nil
		}
	}

	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("eligible=%v", filteredEligible), mergeContext)
	}

	if forceMergeRunning {
		return nil, nil
	}

	if maxSegmentCount == 1 && totalMergeBytes < maxMergeBytes {
		spec := NewMergeSpecification()
		allOfThem := make([]*SegmentCommitInfo, 0, len(filteredEligible))
		for _, segSizeDocs := range filteredEligible {
			allOfThem = append(allOfThem, segSizeDocs.segInfo)
		}
		spec.Add(NewOneMerge(allOfThem))
		return spec, nil
	}

	var spec *MergeSpecification
	index := len(filteredEligible) - 1
	resultingSegments := len(filteredEligible)

	for {
		candidate := make([]*SegmentCommitInfo, 0)
		var currentCandidateBytes int64
		for index >= 0 && resultingSegments > maxSegmentCount {
			current := filteredEligible[index].segInfo
			initialCandidateSize := len(candidate)
			currentSegmentSize := current.SizeInBytes()

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
		candidateSize := len(candidate)
		if candidateSize > 1 && (!forceMergeRunning || float64(currentCandidateBytes) > 0.7*float64(maxMergeBytes)) {
			merge := NewOneMerge(candidate)
			if p.Verbose(mergeContext) {
				p.Message(fmt.Sprintf("add merge=%s", p.SegString(mergeContext, merge.Segments)), mergeContext)
			}
			if spec == nil {
				spec = NewMergeSpecification()
			}
			spec.Add(merge)
		} else {
			return spec, nil
		}
	}
}

func (p *TieredMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("findForcedDeletesMerges infos=%s forceMergeDeletesPctAllowed=%.3f", p.SegString(mergeContext, infos.IteratorToSlice()), p.forceMergeDeletesPctAllowed), mergeContext)
	}

	merging := mergeContext.GetMergingSegments()
	haveWork := false
	totalDelCount := 0
	for info := range infos.Iterator() {
		delCount := mergeContext.NumDeletesToMerge(info)
		totalDelCount += delCount
		pctDeletes := 100.0 * float64(delCount) / float64(info.MaxDoc())
		if pctDeletes > p.forceMergeDeletesPctAllowed && !merging[info] {
			haveWork = true
		}
	}

	if !haveWork {
		return nil, nil
	}

	sortedInfos, err := p.getSortedBySegmentSize(infos, mergeContext)
	if err != nil {
		return nil, err
	}

	eligible := make([]*segmentSizeAndDocs, 0, len(sortedInfos))
	for _, segSizeDocs := range sortedInfos {
		pctDeletes := 100.0 * float64(segSizeDocs.delCount) / float64(segSizeDocs.maxDoc)
		if !merging[segSizeDocs.segInfo] && pctDeletes > p.forceMergeDeletesPctAllowed {
			eligible = append(eligible, segSizeDocs)
		}
	}

	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("eligible=%v", eligible), mergeContext)
	}

	return p.doFindMerges(
		eligible,
		p.maxMergedSegmentBytes,
		math.MaxInt32,
		math.MaxInt32,
		0,
		p.getMaxAllowedDocs(infos.TotalMaxDoc(), totalDelCount),
		mergeTypeForceMergeDeletes,
		mergeContext,
		false,
	)
}

func (p *TieredMergePolicy) getMaxAllowedDocs(totalMaxDoc, totalDelDocs int) int {
	liveDocs := totalMaxDoc - totalDelDocs
	return (liveDocs + p.targetSearchConcurrency - 1) / p.targetSearchConcurrency
}

func (p *TieredMergePolicy) floorSize(bytes int64) int64 {
	if p.floorSegmentBytes > bytes {
		return p.floorSegmentBytes
	}
	return bytes
}

func (p *TieredMergePolicy) String() string {
	return fmt.Sprintf("[TieredMergePolicy: maxMergeAtOnce=%d, maxMergedSegmentMB=%.3f, floorSegmentMB=%.3f, forceMergeDeletesPctAllowed=%.3f, segmentsPerTier=%.3f, maxCFSSegmentSizeMB=%.3f, noCFSRatio=%.3f, deletesPctAllowed=%.3f, targetSearchConcurrency=%d]",
		p.maxMergeAtOnce,
		p.GetMaxMergedSegmentMB(),
		p.GetFloorSegmentMB(),
		p.forceMergeDeletesPctAllowed,
		p.segsPerTier,
		p.GetMaxCFSSegmentSizeMB(),
		p.GetNoCFSRatio(),
		p.deletesPctAllowed,
		p.targetSearchConcurrency,
	)
}

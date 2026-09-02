// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TieredMergePolicy merges segments of approximately equal size, subject to an allowed number of segments per tier.
// This is the Go port of Lucene's org.apache.lucene.index.TieredMergePolicy from Apache Lucene 10.5.0.
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

// NewTieredMergePolicy constructs a TieredMergePolicy with default settings.
func NewTieredMergePolicy() *TieredMergePolicy {
	return &TieredMergePolicy{
		BaseMergePolicy:             NewBaseMergePolicy(),
		maxMergeAtOnce:              10,
		maxMergedSegmentBytes:       5 * 1024 * 1024 * 1024,
		floorSegmentBytes:           16 * 1024 * 1024,
		segsPerTier:                 8.0,
		forceMergeDeletesPctAllowed: 10.0,
		deletesPctAllowed:           20.0,
		targetSearchConcurrency:     1,
	}
}

func (p *TieredMergePolicy) SetMaxMergeAtOnce(v int) *TieredMergePolicy {
	if v < 2 {
		panic("maxMergeAtOnce must be > 1")
	}
	p.maxMergeAtOnce = v
	return p
}

func (p *TieredMergePolicy) GetMaxMergeAtOnce() int {
	return p.maxMergeAtOnce
}

func (p *TieredMergePolicy) SetMaxMergedSegmentMB(v float64) *TieredMergePolicy {
	if v < 0.0 {
		panic("maxMergedSegmentMB must be >= 0")
	}
	p.maxMergedSegmentBytes = int64(v * 1024 * 1024)
	return p
}

func (p *TieredMergePolicy) GetMaxMergedSegmentMB() float64 {
	return float64(p.maxMergedSegmentBytes) / 1024.0 / 1024.0
}

func (p *TieredMergePolicy) SetDeletesPctAllowed(v float64) *TieredMergePolicy {
	if v <= 0 || v > 50 {
		panic("deletesPctAllowed must be > 0 and <= 50")
	}
	p.deletesPctAllowed = v
	return p
}

func (p *TieredMergePolicy) GetDeletesPctAllowed() float64 {
	return p.deletesPctAllowed
}

func (p *TieredMergePolicy) SetFloorSegmentMB(v float64) *TieredMergePolicy {
	if v <= 0.0 {
		panic("floorSegmentMB must be > 0.0")
	}
	p.floorSegmentBytes = int64(v * 1024 * 1024)
	return p
}

func (p *TieredMergePolicy) GetFloorSegmentMB() float64 {
	return float64(p.floorSegmentBytes) / (1024 * 1024)
}

func (p *TieredMergePolicy) SetForceMergeDeletesPctAllowed(v float64) *TieredMergePolicy {
	if v < 0.0 || v > 100.0 {
		panic("forceMergeDeletesPctAllowed must be between 0.0 and 100.0 inclusive")
	}
	p.forceMergeDeletesPctAllowed = v
	return p
}

func (p *TieredMergePolicy) GetForceMergeDeletesPctAllowed() float64 {
	return p.forceMergeDeletesPctAllowed
}

func (p *TieredMergePolicy) SetSegmentsPerTier(v float64) *TieredMergePolicy {
	if v < 2.0 {
		panic("segmentsPerTier must be >= 2.0")
	}
	p.segsPerTier = v
	return p
}

func (p *TieredMergePolicy) GetSegmentsPerTier() float64 {
	return p.segsPerTier
}

func (p *TieredMergePolicy) SetTargetSearchConcurrency(v int) *TieredMergePolicy {
	if v < 1 {
		panic("targetSearchConcurrency must be >= 1")
	}
	p.targetSearchConcurrency = v
	return p
}

func (p *TieredMergePolicy) GetTargetSearchConcurrency() int {
	return p.targetSearchConcurrency
}

func (p *TieredMergePolicy) maxFullFlushMergeSize() int64 {
	return p.floorSegmentBytes
}

type segmentSizeAndDocs struct {
	segInfo     *SegmentCommitInfo
	sizeInBytes int64
	delCount    int
	maxDoc      int
	name        string
}

type mergeScore struct {
	score       float64
	explanation string
}

func (p *TieredMergePolicy) getSortedBySegmentSize(infos *SegmentInfos, mergeContext MergeContext) []*segmentSizeAndDocs {
	sorted := make([]*segmentSizeAndDocs, 0, infos.Size())
	for sci := range infos.Iterator() {
		sorted = append(sorted, &segmentSizeAndDocs{
			segInfo:     sci,
			sizeInBytes: sci.SegmentInfo().SizeInBytes(),
			delCount:    mergeContext.NumDeletesToMerge(sci, 0),
			maxDoc:      sci.SegmentInfo().DocCount(),
			name:        sci.SegmentInfo().Name,
		})
	}

	sort.Slice(sorted, func(i, j int) bool {
		cmp := int(sorted[j].sizeInBytes - sorted[i].sizeInBytes)
		if cmp == 0 {
			return sorted[i].name < sorted[j].name
		}
		return cmp > 0
	})

	return sorted
}

func (p *TieredMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	merging := mergeContext.GetMergingSegments()
	var totIndexBytes int64
	var totalDelDocs, totalMaxDoc int
	var mergingBytes int64

	sortedInfos := p.getSortedBySegmentSize(infos, mergeContext)
	eligible := make([]*segmentSizeAndDocs, 0, len(sortedInfos))

	for _, seg := range sortedInfos {
		if merging[seg.segInfo] {
			mergingBytes += seg.sizeInBytes
			totalMaxDoc += seg.maxDoc - seg.delCount
		} else {
			totalDelDocs += seg.delCount
			totalMaxDoc += seg.maxDoc
			eligible = append(eligible, seg)
		}
		totIndexBytes += seg.sizeInBytes
	}

	totalDelPct := 100.0 * float64(totalDelDocs) / float64(totalMaxDoc)
	allowedDelCount := int(p.deletesPctAllowed * float64(totalMaxDoc) / 100.0)

	tooBigCount := 0
	concurrencyCount := 0
	var allowedSegCount float64

	var finalEligible []*segmentSizeAndDocs
	for _, seg := range eligible {
		segDelPct := 100.0 * float64(seg.delCount) / float64(seg.maxDoc)
		if seg.sizeInBytes > p.maxMergedSegmentBytes/2 && (totalDelPct <= p.deletesPctAllowed || segDelPct <= p.deletesPctAllowed) {
			tooBigCount++
			totIndexBytes -= seg.sizeInBytes
			allowedDelCount -= seg.delCount
		} else if concurrencyCount+tooBigCount < p.targetSearchConcurrency-1 {
			concurrencyCount++
			allowedSegCount++
			totIndexBytes -= seg.sizeInBytes
		} else {
			finalEligible = append(finalEligible, seg)
		}
	}
	allowedDelCount = int(math.Max(0, float64(allowedDelCount)))

	mergeFactor := int(math.Min(float64(p.maxMergeAtOnce), p.segsPerTier))
	levelSize := int64(math.Max(float64(p.floorSegmentBytes), 1.0))
	bytesLeft := totIndexBytes

	for {
		segCountLevel := float64(bytesLeft) / float64(levelSize)
		if segCountLevel < p.segsPerTier || levelSize == p.maxMergedSegmentBytes {
			allowedSegCount += math.Ceil(segCountLevel)
			break
		}
		allowedSegCount += p.segsPerTier
		bytesLeft -= int64(p.segsPerTier) * levelSize
		levelSize = int64(math.Min(float64(p.maxMergedSegmentBytes), float64(levelSize)*float64(mergeFactor)))
	}
	allowedSegCount = math.Max(allowedSegCount, p.segsPerTier)
	allowedSegCount = math.Max(allowedSegCount, float64(p.targetSearchConcurrency-tooBigCount))

	allowedDocCount := int(math.Ceil(float64(totalMaxDoc-totalDelDocs) / float64(p.targetSearchConcurrency)))

	return p.doFindMerges(
		finalEligible,
		p.maxMergedSegmentBytes,
		mergeFactor,
		int(allowedSegCount),
		allowedDelCount,
		allowedDocCount,
		"NATURAL",
		mergeContext,
		mergingBytes >= p.maxMergedSegmentBytes,
	)
}

func (p *TieredMergePolicy) doFindMerges(
	sortedEligible []*segmentSizeAndDocs,
	maxMergedSegmentBytes int64,
	mergeFactor int,
	allowedSegCount int,
	allowedDelCount int,
	allowedDocCount int,
	mergeType string,
	mergeContext MergeContext,
	maxMergeIsRunning bool,
) (*MergeSpecification, error) {
	sortedEligibleCopy := make([]*segmentSizeAndDocs, len(sortedEligible))
	copy(sortedEligibleCopy, sortedEligible)

	segInfosSizes := make(map[*SegmentCommitInfo]*segmentSizeAndDocs)
	for _, seg := range sortedEligible {
		segInfosSizes[seg.segInfo] = seg
	}

	toBeMerged := make(map[*SegmentCommitInfo]bool)
	var spec *MergeSpecification
	haveOneLargeMerge := false

	for {
		eligible := make([]*segmentSizeAndDocs, 0)
		for _, seg := range sortedEligibleCopy {
			if !toBeMerged[seg.segInfo] {
				eligible = append(eligible, seg)
			}
		}

		if len(eligible) == 0 {
			return spec, nil
		}

		remainingDelCount := 0
		for _, seg := range eligible {
			remainingDelCount += seg.delCount
		}

		if mergeType == "NATURAL" && len(eligible) <= allowedSegCount && remainingDelCount <= allowedDelCount {
			return spec, nil
		}

		var best []*SegmentCommitInfo
		var bestScore *mergeScore
		bestTooLarge := false
		var bestMergeBytes int64

		for startIdx := 0; startIdx < len(eligible); startIdx++ {
			candidate := make([]*SegmentCommitInfo, 0)
			hitTooLarge := false
			var bytesThisMerge int64
			var docCountThisMerge int

			for idx := startIdx; idx < len(eligible) && len(candidate) < p.maxMergeAtOnce; idx++ {
				seg := eligible[idx]
				if bytesThisMerge+seg.sizeInBytes > maxMergedSegmentBytes ||
					(bytesThisMerge > p.floorSegmentBytes && docCountThisMerge+seg.maxDoc-seg.delCount > allowedDocCount) {
					hitTooLarge = hitTooLarge || (bytesThisMerge+seg.sizeInBytes > maxMergedSegmentBytes)
					if len(candidate) > 0 {
						continue
					}
				}
				candidate = append(candidate, seg.segInfo)
				bytesThisMerge += seg.sizeInBytes
				docCountThisMerge += seg.maxDoc - seg.delCount
				if len(candidate) >= mergeFactor && bytesThisMerge >= p.floorSegmentBytes {
					// potentially a good merge
				}
			}

			if len(candidate) == 0 {
				continue
			}

			maxCandidateSeg := segInfosSizes[candidate[0]]
			if !hitTooLarge && mergeType == "NATURAL" && bytesThisMerge < int64(float64(maxCandidateSeg.sizeInBytes)*1.5) &&
				float64(maxCandidateSeg.delCount) < float64(maxCandidateSeg.maxDoc)*p.deletesPctAllowed/100.0 {
				continue
			}

			if len(candidate) == 1 && maxCandidateSeg.delCount == 0 {
				continue
			}

			if bestScore != nil && !hitTooLarge && len(candidate) < mergeFactor {
				break
			}

			score := p.score(candidate, hitTooLarge, segInfosSizes)
			if bestScore == nil || score.score < bestScore.score {
				if !hitTooLarge || !maxMergeIsRunning {
					best = candidate
					bestScore = score
					bestTooLarge = hitTooLarge
					bestMergeBytes = bytesThisMerge
				}
			}
		}

		if best == nil {
			return spec, nil
		}

		if !haveOneLargeMerge || !bestTooLarge || mergeType == "FORCE_MERGE_DELETES" {
			haveOneLargeMerge = haveOneLargeMerge || bestTooLarge
			if spec == nil {
				spec = NewMergeSpecification()
			}
			spec.Add(NewOneMerge(best))
		}
		for _, seg := range best {
			toBeMerged[seg] = true
		}
	}
}

func (p *TieredMergePolicy) score(candidate []*SegmentCommitInfo, hitTooLarge bool, segInfosSizes map[*SegmentCommitInfo]*segmentSizeAndDocs) *mergeScore {
	var totBeforeMergeBytes int64
	var totAfterMergeBytes int64
	var totAfterMergeBytesFloored int64
	for _, info := range candidate {
		seg := segInfosSizes[info]
		totAfterMergeBytes += seg.sizeInBytes
		totAfterMergeBytesFloored += int64(math.Max(float64(p.floorSegmentBytes), float64(seg.sizeInBytes)))
		totBeforeMergeBytes += info.SegmentInfo().SizeInBytes()
	}

	var skew float64
	if hitTooLarge {
		mergeFactor := int(math.Min(float64(p.maxMergeAtOnce), p.segsPerTier))
		skew = 1.0 / float64(mergeFactor)
	} else {
		skew = float64(int64(math.Max(float64(p.floorSegmentBytes), float64(segInfosSizes[candidate[0]].sizeInBytes)))) / float64(totAfterMergeBytesFloored)
	}

	mergeScore := skew
	mergeScore *= math.Pow(float64(totAfterMergeBytes), 0.05)
	nonDelRatio := float64(totAfterMergeBytes) / float64(totBeforeMergeBytes)
	mergeScore *= math.Pow(nonDelRatio, 2)

	return &mergeScore{
		score: mergeScore,
		explanation: fmt.Sprintf("skew=%.3f nonDelRatio=%.3f", skew, nonDelRatio),
	}
}

func (p *TieredMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	sortedInfos := p.getSortedBySegmentSize(infos, mergeContext)
	var totalMergeBytes int64
	merging := mergeContext.GetMergingSegments()

	eligible := make([]*segmentSizeAndDocs, 0)
	for _, seg := range sortedInfos {
		if segmentsToMerge[seg.segInfo] != nil {
			if merging[seg.segInfo] {
				eligible = append(eligible, seg)
			} else {
				totalMergeBytes += seg.sizeInBytes
			}
		}
	}

	maxMergeBytes := p.maxMergedSegmentBytes
	if maxSegmentCount == 1 {
		maxMergeBytes = math.MaxInt64
	} else if maxSegmentCount != math.MaxInt32 {
		maxMergeBytes = int64(math.Max(float64(totalMergeBytes)/float64(maxSegmentCount), float64(p.maxMergedSegmentBytes)) * 1.25)
	}

	var spec *MergeSpecification
	index := len(eligible) - 1
	resultingSegments := len(eligible)

	for index >= 0 && resultingSegments > maxSegmentCount {
		candidate := make([]*SegmentCommitInfo, 0)
		var currentCandidateBytes int64
		for index >= 0 && resultingSegments > maxSegmentCount {
			seg := eligible[index]
			if currentCandidateBytes+seg.sizeInBytes <= maxMergeBytes || len(candidate) < 2 {
				candidate = append(candidate, seg.segInfo)
				index--
				currentCandidateBytes += seg.sizeInBytes
				if len(candidate) > 1 {
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

func (p *TieredMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	merging := mergeContext.GetMergingSegments()
	var totalDelCount int
	for sci := range infos.Iterator() {
		totalDelCount += mergeContext.NumDeletesToMerge(sci, 0)
	}

	sortedInfos := p.getSortedBySegmentSize(infos, mergeContext)
	eligible := make([]*segmentSizeAndDocs, 0)
	for _, seg := range sortedInfos {
		pctDeletes := 100.0 * float64(seg.delCount) / float64(seg.maxDoc)
		if !merging[seg.segInfo] && pctDeletes > p.forceMergeDeletesPctAllowed {
			eligible = append(eligible, seg)
		}
	}

	return p.doFindMerges(
		eligible,
		p.maxMergedSegmentBytes,
		math.MaxInt32,
		math.MaxInt32,
		0,
		int(math.Ceil(float64(infos.totalMaxDoc()-totalDelCount)/float64(p.targetSearchConcurrency))),
		"FORCE_MERGE_DELETES",
		mergeContext,
		false,
	)
}

func (p *TieredMergePolicy) String() string {
	return fmt.Sprintf("[TieredMergePolicy: maxMergeAtOnce=%d, maxMergedSegmentMB=%.1f, floorSegmentMB=%.1f, forceMergeDeletesPctAllowed=%.1f, segmentsPerTier=%.1f, deletesPctAllowed=%.1f, targetSearchConcurrency=%d]",
		p.maxMergeAtOnce, p.GetMaxMergedSegmentMB(), p.GetFloorSegmentMB(), p.forceMergeDeletesPctAllowed, p.segsPerTier, p.deletesPctAllowed, p.targetSearchConcurrency)
}

var _ MergePolicy = (*TieredMergePolicy)(nil)

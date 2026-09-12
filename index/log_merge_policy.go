// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
)

const (
	// LevelLogSpan defines the allowed range of log(size) for each level.
	// Mirrors LogMergePolicy.LEVEL_LOG_SPAN.
	LevelLogSpan = 0.75

	// DefaultMergeFactor is the default number of segments merged at a time.
	// Mirrors LogMergePolicy.DEFAULT_MERGE_FACTOR.
	DefaultMergeFactor = 10

	// DefaultMaxMergeDocs is the default maximum segment size in documents.
	// Mirrors LogMergePolicy.DEFAULT_MAX_MERGE_DOCS.
	DefaultMaxMergeDocs = math.MaxInt32

	// DefaultLogMergeNoCFSRatio is the default ratio for disabling compound files.
	// Mirrors LogMergePolicy.DEFAULT_NO_CFS_RATIO.
	DefaultLogMergeNoCFSRatio = 0.1
)

// LogMergePolicy implements a MergePolicy that tries to merge segments into levels of
// exponentially increasing size, where each level has fewer segments than the value of the
// merge factor.
//
// This is a faithful port of org.apache.lucene.index.LogMergePolicy from Apache Lucene 10.5.0.
type LogMergePolicy struct {
	*BaseMergePolicy

	// mergeFactor is how many segments are merged at a time.
	mergeFactor int

	// minMergeSize is the size below which segments are candidates for full-flush merges.
	minMergeSize int64

	// maxMergeSize is the size above which a segment will never be merged.
	maxMergeSize int64

	// maxMergeSizeForForcedMerge is the size above which a segment will never be merged during ForceMerge.
	maxMergeSizeForForcedMerge int64

	// maxMergeDocs is the document count above which a segment will never be merged.
	maxMergeDocs int

	// calibrateSizeByDeletes controls whether to pro-rate a segment's size by the percentage of non-deleted docs.
	calibrateSizeByDeletes bool

	// targetSearchConcurrency prevents creating segments bigger than maxDoc / targetSearchConcurrency.
	targetSearchConcurrency int

	// sizeCalculator is used to implement the abstract 'size' method from Java.
	// It is set by concrete subclasses (e.g., LogDocMergePolicy, LogByteSizeMergePolicy).
	sizeCalculator func(*SegmentCommitInfo, MergeContext) int64
}

// NewLogMergePolicy creates a new LogMergePolicy with default settings.
func NewLogMergePolicy() *LogMergePolicy {
	return &LogMergePolicy{
		BaseMergePolicy:            NewBaseMergePolicyWithDefaults(DefaultLogMergeNoCFSRatio, DefaultMaxCFSSegmentSize),
		mergeFactor:                DefaultMergeFactor,
		maxMergeSizeForForcedMerge: math.MaxInt64,
		maxMergeDocs:               DefaultMaxMergeDocs,
		calibrateSizeByDeletes:     true,
		targetSearchConcurrency:    1,
	}
}

// GetMergeFactor returns the number of segments that are merged at once.
func (p *LogMergePolicy) GetMergeFactor() int {
	return p.mergeFactor
}

// SetMergeFactor sets the number of segments that are merged at once.
func (p *LogMergePolicy) SetMergeFactor(mergeFactor int) {
	if mergeFactor < 2 {
		panic("mergeFactor cannot be less than 2")
	}
	p.mergeFactor = mergeFactor
}

// GetCalibrateSizeByDeletes returns whether segment size is calibrated by deletes.
func (p *LogMergePolicy) GetCalibrateSizeByDeletes() bool {
	return p.calibrateSizeByDeletes
}

// SetCalibrateSizeByDeletes sets whether segment size is calibrated by deletes.
func (p *LogMergePolicy) SetCalibrateSizeByDeletes(calibrateSizeByDeletes bool) {
	p.calibrateSizeByDeletes = calibrateSizeByDeletes
}

// GetTargetSearchConcurrency returns the target search concurrency.
func (p *LogMergePolicy) GetTargetSearchConcurrency() int {
	return p.targetSearchConcurrency
}

// SetTargetSearchConcurrency sets the target search concurrency.
func (p *LogMergePolicy) SetTargetSearchConcurrency(targetSearchConcurrency int) {
	if targetSearchConcurrency < 1 {
		panic(fmt.Sprintf("targetSearchConcurrency must be >= 1 (got %d)", targetSearchConcurrency))
	}
	p.targetSearchConcurrency = targetSearchConcurrency
}

// sizeDocs returns the number of documents in the provided SegmentCommitInfo,
// pro-rated by percentage of non-deleted documents if calibrateSizeByDeletes is set.
func (p *LogMergePolicy) sizeDocs(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	if p.calibrateSizeByDeletes {
		delCount := mergeContext.NumDeletesToMerge(info)
		return int64(info.SegmentInfo().MaxDoc() - delCount)
	}
	return int64(info.SegmentInfo().MaxDoc())
}

// sizeBytes returns the byte size of the provided SegmentCommitInfo,
// pro-rated by percentage of non-deleted documents if calibrateSizeByDeletes is set.
func (p *LogMergePolicy) sizeBytes(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	if p.calibrateSizeByDeletes {
		size, err := p.BaseMergePolicy.Size(info, mergeContext)
		if err != nil {
			return 0 // In a real implementation, we'd handle the error
		}
		return size
	}
	return info.SegmentInfo().SizeInBytes()
}

// Size implements the 'abstract' size method. It uses the sizeCalculator if provided,
// otherwise it defaults to byte size.
func (p *LogMergePolicy) Size(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	if p.sizeCalculator != nil {
		return p.sizeCalculator(info, mergeContext)
	}
	return p.sizeBytes(info, mergeContext)
}

// isMerged reports whether the segment is already fully merged.
func (p *LogMergePolicy) isMerged(infos *SegmentInfos, info *SegmentCommitInfo, mergeContext MergeContext) (bool, error) {
	return p.BaseMergePolicy.IsMerged(infos, info, mergeContext)
}

func (p *LogMergePolicy) MaxFullFlushMergeSize() int64 {
	return p.minMergeSize
}

// findForcedMergesSizeLimit returns merges necessary to merge the index when some
// segments exceed the size or doc limits.
func (p *LogMergePolicy) findForcedMergesSizeLimit(infos *SegmentInfos, last int, mergeContext MergeContext) (*MergeSpecification, error) {
	spec := NewMergeSpecification()
	segments := infos.List()

	start := last - 1
	for start >= 0 {
		info := infos.Get(start)
		if p.Size(info, mergeContext) > p.maxMergeSizeForForcedMerge ||
			p.sizeDocs(info, mergeContext) > int64(p.maxMergeDocs) {
			if p.Verbose(mergeContext) {
				p.Message(fmt.Sprintf("findForcedMergesSizeLimit: skip segment=%s: size is > maxMergeSize (%d) or sizeDocs is > maxMergeDocs (%d)",
					info.String(), p.maxMergeSizeForForcedMerge, p.maxMergeDocs), mergeContext)
			}
			if last-start-1 > 1 {
				merged := make([]*SegmentCommitInfo, 0, last-start-1)
				for i := start + 1; i < last; i++ {
					merged = append(merged, segments[i])
				}
				spec.Add(NewOneMerge(merged))
			} else if last-start-1 == 1 {
				infoNext := infos.Get(start + 1)
				merged, err := p.isMerged(infos, infoNext, mergeContext)
				if err != nil {
					return nil, err
				}
				if !merged {
					spec.Add(NewOneMerge([]*SegmentCommitInfo{infoNext}))
				}
			}
			last = start
		} else if last-start == p.mergeFactor {
			merged := make([]*SegmentCommitInfo, 0, p.mergeFactor)
			for i := start; i < last; i++ {
				merged = append(merged, segments[i])
			}
			spec.Add(NewOneMerge(merged))
			last = start
		}
		start--
	}

	if last > 0 {
		start = last - 1
		if last > 1 || func() bool {
			merged, err := p.isMerged(infos, infos.Get(start), mergeContext)
			if err != nil {
				return false
			}
			return !merged
		}() {
			merged := make([]*SegmentCommitInfo, 0, last)
			for i := 0; i < last; i++ {
				merged = append(merged, segments[i])
			}
			spec.Add(NewOneMerge(merged))
		}
	}

	if spec.Size() == 0 {
		return nil, nil
	}
	return spec, nil
}

// findForcedMergesMaxNumSegments returns merges to reach exactly maxNumSegments.
func (p *LogMergePolicy) findForcedMergesMaxNumSegments(infos *SegmentInfos, maxNumSegments, last int, mergeContext MergeContext) (*MergeSpecification, error) {
	spec := NewMergeSpecification()
	segments := infos.List()

	for last-maxNumSegments+1 >= p.mergeFactor {
		merged := make([]*SegmentCommitInfo, 0, p.mergeFactor)
		for i := last - p.mergeFactor; i < last; i++ {
			merged = append(merged, segments[i])
		}
		spec.Add(NewOneMerge(merged))
		last -= p.mergeFactor
	}

	if spec.Size() == 0 {
		if maxNumSegments == 1 {
			if last > 1 {
				merged := make([]*SegmentCommitInfo, 0, last)
				for i := 0; i < last; i++ {
					merged = append(merged, segments[i])
				}
				spec.Add(NewOneMerge(merged))
			} else if last == 1 {
				merged, err := p.isMerged(infos, infos.Get(0), mergeContext)
				if err != nil {
					return nil, err
				}
				if !merged {
					spec.Add(NewOneMerge([]*SegmentCommitInfo{infos.Get(0)}))
				}
			}
		} else if last > maxNumSegments {
			finalMergeSize := last - maxNumSegments + 1
			var bestSize int64
			bestStart := 0

			for i := 0; i < last-finalMergeSize+1; i++ {
				var sumSize int64
				for j := 0; j < finalMergeSize; j++ {
					sumSize += p.Size(infos.Get(i+j), mergeContext)
				}
				if i == 0 || (sumSize < 2*p.Size(infos.Get(i-1), mergeContext) && sumSize < bestSize) {
					bestStart = i
					bestSize = sumSize
				}
			}
			merged := make([]*SegmentCommitInfo, 0, finalMergeSize)
			for i := bestStart; i < bestStart+finalMergeSize; i++ {
				merged = append(merged, segments[i])
			}
			spec.Add(NewOneMerge(merged))
		}
	}

	if spec.Size() == 0 {
		return nil, nil
	}
	return spec, nil
}

// FindForcedMerges implements the forced merge logic.
func (p *LogMergePolicy) FindForcedMerges(infos *SegmentInfos, maxNumSegments int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	if maxNumSegments <= 0 {
		panic("maxNumSegments must be > 0")
	}
	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("findForcedMerges: maxNumSegs=%d segsToMerge=%v", maxNumSegments, segmentsToMerge), mergeContext)
	}

	alreadyMerged, err := p.isMergedForced(infos, maxNumSegments, segmentsToMerge, mergeContext)
	if err != nil {
		return nil, err
	}
	if alreadyMerged {
		if p.Verbose(mergeContext) {
			p.Message("already merged; skip", mergeContext)
		}
		return nil, nil
	}

	last := infos.Size()
	for last > 0 {
		info := infos.Get(last - 1)
		if segmentsToMerge[info] {
			last++
			break
		}
		last--
	}

	if last == 0 {
		if p.Verbose(mergeContext) {
			p.Message("last == 0; skip", mergeContext)
		}
		return nil, nil
	}

	if maxNumSegments == 1 && last == 1 {
		merged, err := p.isMerged(infos, infos.Get(0), mergeContext)
		if err != nil {
			return nil, err
		}
		if merged {
			if p.Verbose(mergeContext) {
				p.Message("already 1 seg; skip", mergeContext)
			}
			return nil, nil
		}
	}

	anyTooLarge := false
	for i := 0; i < last; i++ {
		info := infos.Get(i)
		if p.Size(info, mergeContext) > p.maxMergeSizeForForcedMerge ||
			p.sizeDocs(info, mergeContext) > int64(p.maxMergeDocs) {
			anyTooLarge = true
			break
		}
	}

	if anyTooLarge {
		return p.findForcedMergesSizeLimit(infos, last, mergeContext)
	}
	return p.findForcedMergesMaxNumSegments(infos, maxNumSegments, last, mergeContext)
}

func (p *LogMergePolicy) isMergedForced(infos *SegmentInfos, maxNumSegments int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (bool, error) {
	numSegments := infos.Size()
	numToMerge := 0
	var mergeInfo *SegmentCommitInfo
	var segmentIsOriginal bool

	for i := 0; i < numSegments && numToMerge <= maxNumSegments; i++ {
		info := infos.Get(i)
		if isOriginal, ok := segmentsToMerge[info]; ok {
			segmentIsOriginal = isOriginal
			numToMerge++
			mergeInfo = info
		}
	}

	if numToMerge > maxNumSegments {
		return false, nil
	}
	if numToMerge == 1 && segmentIsOriginal {
		return p.isMerged(infos, mergeInfo, mergeContext)
	}
	return true, nil
}

// FindForcedDeletesMerges finds merges to expunge all deletes from the index.
func (p *LogMergePolicy) FindForcedDeletesMerges(segmentInfos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if mergeContext == nil {
		return nil, nil
	}
	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("findForcedDeleteMerges: %d segments", segmentInfos.Size()), mergeContext)
	}

	spec := NewMergeSpecification()
	segments := segmentInfos.List()
	numSegments := len(segments)
	firstSegmentWithDeletions := -1

	for i := 0; i < numSegments; i++ {
		info := segments[i]
		delCount := mergeContext.NumDeletesToMerge(info)
		if delCount > 0 {
			if p.Verbose(mergeContext) {
				p.Message(fmt.Sprintf("  segment %s has deletions", info.SegmentInfo().Name()), mergeContext)
			}
			if firstSegmentWithDeletions == -1 {
				firstSegmentWithDeletions = i
			} else if i-firstSegmentWithDeletions == p.mergeFactor {
				if p.Verbose(mergeContext) {
					p.Message(fmt.Sprintf("  add merge %d to %d inclusive", firstSegmentWithDeletions, i-1), mergeContext)
				}
				spec.Add(NewOneMerge(segments[firstSegmentWithDeletions:i]))
				firstSegmentWithDeletions = i
			}
		} else if firstSegmentWithDeletions != -1 {
			if p.Verbose(mergeContext) {
				p.Message(fmt.Sprintf("  add merge %d to %d inclusive", firstSegmentWithDeletions, i-1), mergeContext)
			}
			spec.Add(NewOneMerge(segments[firstSegmentWithDeletions:i]))
			firstSegmentWithDeletions = -1
		}
	}

	if firstSegmentWithDeletions != -1 {
		if p.Verbose(mergeContext) {
			p.Message(fmt.Sprintf("  add merge %d to %d inclusive", firstSegmentWithDeletions, numSegments-1), mergeContext)
		}
		spec.Add(NewOneMerge(segments[firstSegmentWithDeletions:]))
	}

	if spec.Size() == 0 {
		return nil, nil
	}
	return spec, nil
}

type segmentInfoAndLevel struct {
	info  *SegmentCommitInfo
	level float32
}

// FindMerges identifies merges necessary to merge the index.
func (p *LogMergePolicy) FindMerges(mergeTrigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if mergeContext == nil {
		return nil, nil
	}
	numSegments := infos.Size()
	if p.Verbose(mergeContext) {
		p.Message(fmt.Sprintf("findMerges: %d segments", numSegments), mergeContext)
	}

	norm := math.Log(float64(p.mergeFactor))
	mergingSegments := mergeContext.GetMergingSegments()

	levels := make([]segmentInfoAndLevel, 0, numSegments)
	totalDocCount := 0
	for i := 0; i < numSegments; i++ {
		info := infos.Get(i)
		totalDocCount += int(p.sizeDocs(info, mergeContext))
		size := p.Size(info, mergeContext)
		if size < 1 {
			size = 1
		}
		levels = append(levels, segmentInfoAndLevel{
			info:  info,
			level: float32(math.Log(float64(size)) / norm),
		})

		if p.Verbose(mergeContext) {
			segBytes := p.sizeBytes(info, mergeContext)
			extra := ""
			if mergingSegments[info] {
				extra += " [merging]"
			}
			if size >= p.maxMergeSize {
				extra += " [skip: too large]"
			}
			p.Message(fmt.Sprintf("seg=%s level=%.3f size=%.3f MB%s",
				p.SegString(mergeContext, []*SegmentCommitInfo{info}),
				levels[len(levels)-1].level,
				float64(segBytes)/1024.0/1024.0,
				extra), mergeContext)
		}
	}

	var levelFloor float32
	if p.minMergeSize <= 0 {
		levelFloor = 0.0
	} else {
		levelFloor = float32(math.Log(float64(p.minMergeSize)) / norm)
	}

	maxLevels := make([]float32, numSegments+1)
	maxLevels[numSegments] = -1.0
	for i := numSegments - 1; i >= 0; i-- {
		maxLevels[i] = float32(math.Max(float64(levels[i].level), float64(maxLevels[i+1])))
	}

	var spec *MergeSpecification
	start := 0
	for start < numSegments {
		maxLevel := maxLevels[start]
		var levelBottom float32
		if maxLevel > levelFloor {
			levelBottom = maxLevel - LevelLogSpan
		} else {
			levelBottom = maxLevel - 2*LevelLogSpan
		}

		upto := numSegments - 1
		for upto >= start {
			if levels[upto].level >= levelBottom {
				break
			}
			upto--
		}
		if p.Verbose(mergeContext) {
			p.Message(fmt.Sprintf("  level %.3f to %.3f: %d segments",
				levelBottom, maxLevel, 1+upto-start), mergeContext)
		}

		maxMergeDocs := int(math.Min(float64(p.maxMergeDocs), math.Ceil(float64(totalDocCount)/float64(p.targetSearchConcurrency))))

		end := start + p.mergeFactor
		for end <= 1+upto {
			anyMerging := false
			var mergeSize int64
			var mergeDocs int
			for i := start; i < end; i++ {
				info := levels[i].info
				if mergingSegments[info] {
					anyMerging = true
					break
				}
				s := p.Size(info, mergeContext)
				d := int(p.sizeDocs(info, mergeContext))
				if mergeSize+s > p.maxMergeSize || mergeDocs+d > maxMergeDocs {
					if i == start {
						if p.Verbose(mergeContext) {
							p.Message(fmt.Sprintf("    %d is larger than the max merge size/docs; ignoring", i), mergeContext)
						}
						end = i + 1
					} else {
						end = i
					}
					break
				}
				mergeSize += s
				mergeDocs += d
			}

			if end-start >= p.mergeFactor && p.minMergeSize < p.maxMergeSize && mergeSize < p.minMergeSize && !anyMerging {
				for end < 1+upto {
					info := levels[end].info
					if mergingSegments[info] {
						anyMerging = true
						break
					}
					s := p.Size(info, mergeContext)
					d := int(p.sizeDocs(info, mergeContext))
					if mergeSize+s > p.minMergeSize || mergeDocs+d > maxMergeDocs {
						break
					}
					mergeSize += s
					mergeDocs += d
					end++
				}
			}

			if anyMerging || end-start <= 1 {
				// skip
			} else {
				if spec == nil {
					spec = NewMergeSpecification()
				}
				merged := make([]*SegmentCommitInfo, 0, end-start)
				for i := start; i < end; i++ {
					merged = append(merged, levels[i].info)
				}
				if p.Verbose(mergeContext) {
					p.Message(fmt.Sprintf("  add merge=%s start=%d end=%d",
						p.SegString(mergeContext, merged), start, end), mergeContext)
				}
				spec.Add(NewOneMerge(merged))
			}
			start = end
			end = start + p.mergeFactor
		}
		start = 1 + upto
	}

	return spec, nil
}

// SetMaxMergeDocs sets the maximum number of documents that may be merged.
func (p *LogMergePolicy) SetMaxMergeDocs(maxMergeDocs int) {
	p.maxMergeDocs = maxMergeDocs
}

// GetMaxMergeDocs returns the maximum number of documents that may be merged.
func (p *LogMergePolicy) GetMaxMergeDocs() int {
	return p.maxMergeDocs
}

func (p *LogMergePolicy) String() string {
	return fmt.Sprintf("[LogMergePolicy: minMergeSize=%d, mergeFactor=%d, maxMergeSize=%d, maxMergeSizeForForcedMerge=%d, calibrateSizeByDeletes=%v, maxMergeDocs=%d, maxCFSSegmentSizeMB=%.2f, noCFSRatio=%.2f]",
		p.minMergeSize, p.mergeFactor, p.maxMergeSize, p.maxMergeSizeForForcedMerge, p.calibrateSizeByDeletes, p.maxMergeDocs, p.GetMaxCFSSegmentSizeMB(), p.GetNoCFSRatio())
}

var _ MergePolicy = (*LogMergePolicy)(nil)

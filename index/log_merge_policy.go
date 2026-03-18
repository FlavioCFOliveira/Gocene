// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"
)

// LogMergePolicy is a merge policy that merges segments of approximately equal size
// using a logarithmic tiering approach.
//
// This is the Go port of Lucene's org.apache.lucene.index.LogMergePolicy.
//
// LogMergePolicy is a legacy merge policy that is less efficient than TieredMergePolicy
// but is kept for backward compatibility. It merges the smallest segments first,
// using a logarithmic approach to determine which segments to merge.
//
// The policy works by:
//   - Computing the level of each segment based on its size
//   - Grouping segments into levels
//   - Merging segments within the same level when there are enough of them
//
// This implements GC-636: LogMergePolicy base class
type LogMergePolicy struct {
	*BaseMergePolicy

	// minMergeSize is the minimum segment size to consider for merging (in bytes).
	// Segments smaller than this are treated as this size.
	// Default is 1.6 MB.
	minMergeSize int64

	// maxMergeSize is the maximum segment size to consider for merging (in bytes).
	// Segments larger than this are not merged unless forced.
	// Default is 2048 MB (2 GB).
	maxMergeSize int64

	// maxMergeSizeForForcedMerge is the maximum segment size for forced merges.
	// Default is 0 (unlimited).
	maxMergeSizeForForcedMerge int64

	// mergeFactor is the number of segments to merge at once.
	// Default is 10.
	mergeFactor int

	// noCFSRatio is the ratio for using compound files.
	// If merged segment size / total index size < this ratio, use CFS.
	// Default is 0.0 (always use CFS).
	noCFSRatio float64

	// maxMergeDocs is the maximum number of documents to merge at once.
	// Default is max int.
	maxMergeDocs int

	// calibrateSizeByDeletes controls whether to calibrate segment size by deletes.
	// Default is true.
	calibrateSizeByDeletes bool

	// levelSize is the size multiplier between levels.
	// Default is mergeFactor.
	levelSize int64
}

// NewLogMergePolicy creates a new LogMergePolicy with default settings.
// This implements GC-636: LogMergePolicy constructor
func NewLogMergePolicy() *LogMergePolicy {
	return &LogMergePolicy{
		BaseMergePolicy:            NewBaseMergePolicy(),
		minMergeSize:               1677721, // 1.6 MB
		maxMergeSize:               2048 * 1024 * 1024,              // 2 GB
		maxMergeSizeForForcedMerge: 0,                               // unlimited
		mergeFactor:                10,
		noCFSRatio:                 0.0,
		maxMergeDocs:               math.MaxInt32,
		calibrateSizeByDeletes:     true,
		levelSize:                  10,
	}
}

// GetMinMergeMB returns the minimum merge size in MB.
func (p *LogMergePolicy) GetMinMergeMB() float64 {
	return float64(p.minMergeSize) / 1024.0 / 1024.0
}

// SetMinMergeMB sets the minimum merge size in MB.
func (p *LogMergePolicy) SetMinMergeMB(v float64) {
	p.minMergeSize = int64(v * 1024 * 1024)
}

// GetMaxMergeMB returns the maximum merge size in MB.
func (p *LogMergePolicy) GetMaxMergeMB() float64 {
	return float64(p.maxMergeSize) / 1024.0 / 1024.0
}

// SetMaxMergeMB sets the maximum merge size in MB.
func (p *LogMergePolicy) SetMaxMergeMB(v float64) {
	p.maxMergeSize = int64(v * 1024 * 1024)
}

// GetMaxMergeMBForForcedMerge returns the maximum merge size for forced merges in MB.
func (p *LogMergePolicy) GetMaxMergeMBForForcedMerge() float64 {
	return float64(p.maxMergeSizeForForcedMerge) / 1024.0 / 1024.0
}

// SetMaxMergeMBForForcedMerge sets the maximum merge size for forced merges in MB.
func (p *LogMergePolicy) SetMaxMergeMBForForcedMerge(v float64) {
	p.maxMergeSizeForForcedMerge = int64(v * 1024 * 1024)
}

// GetMergeFactor returns the merge factor.
func (p *LogMergePolicy) GetMergeFactor() int {
	return p.mergeFactor
}

// SetMergeFactor sets the merge factor.
func (p *LogMergePolicy) SetMergeFactor(v int) {
	if v < 2 {
		v = 2
	}
	p.mergeFactor = v
	p.levelSize = int64(v)
}

// GetMaxMergeDocs returns the maximum number of documents to merge.
func (p *LogMergePolicy) GetMaxMergeDocs() int {
	return p.maxMergeDocs
}

// SetMaxMergeDocs sets the maximum number of documents to merge.
func (p *LogMergePolicy) SetMaxMergeDocs(v int) {
	p.maxMergeDocs = v
}

// GetCalibrateSizeByDeletes returns whether to calibrate size by deletes.
func (p *LogMergePolicy) GetCalibrateSizeByDeletes() bool {
	return p.calibrateSizeByDeletes
}

// SetCalibrateSizeByDeletes sets whether to calibrate size by deletes.
func (p *LogMergePolicy) SetCalibrateSizeByDeletes(v bool) {
	p.calibrateSizeByDeletes = v
}

// GetNoCFSRatio returns the compound file ratio.
func (p *LogMergePolicy) GetNoCFSRatio() float64 {
	return p.noCFSRatio
}

// SetNoCFSRatio sets the compound file ratio.
func (p *LogMergePolicy) SetNoCFSRatio(v float64) {
	p.noCFSRatio = v
}

// Size returns the size of a segment for merge policy purposes.
// This may be calibrated by deletes if calibrateSizeByDeletes is true.
func (p *LogMergePolicy) Size(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	byteSize := info.SegmentInfo().SizeInBytes()

	if p.calibrateSizeByDeletes && mergeContext != nil {
		delCount := mergeContext.NumDeletesToMerge(info)
		maxDoc := info.SegmentInfo().DocCount()

		if maxDoc > 0 && delCount > 0 {
			delRatio := float64(delCount) / float64(maxDoc)
			if delRatio > 1.0 {
				delRatio = 1.0
			}
			byteSize = int64(float64(byteSize) * (1.0 - delRatio))
		}
	}

	return byteSize
}

// logSegInfo holds segment info with size for level grouping.
type logSegInfo struct {
	info *SegmentCommitInfo
	size int64
}

// FindMerges finds merges based on the log policy.
// This implements GC-636: LogMergePolicy.FindMerges
func (p *LogMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if mergeContext == nil {
		return nil, nil
	}

	merging := mergeContext.GetMergingSegments()

	// Collect eligible segments (not currently merging)
	eligible := make([]logSegInfo, 0, infos.Size())

	for sci := range infos.Iterator() {
		if !merging[sci] {
			size := p.Size(sci, mergeContext)
			eligible = append(eligible, logSegInfo{info: sci, size: size})
		}
	}

	if len(eligible) == 0 {
		return nil, nil
	}

	// Sort by size (smallest first)
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].size < eligible[j].size
	})

	// Find merges
	spec := NewMergeSpecification()

	// Group segments by level
	levels := p.getLevelSizes(eligible)

	for _, level := range levels {
		// Find merges within this level
		for i := 0; i+p.mergeFactor <= len(level); {
			// Check if we have mergeFactor segments of similar size
			candidate := make([]*SegmentCommitInfo, 0, p.mergeFactor)
			var totalSize int64

			for j := i; j < len(level) && len(candidate) < p.mergeFactor; j++ {
				size := level[j].size
				if totalSize+size > p.maxMergeSize && len(candidate) >= 2 {
					// Would exceed max merge size
					break
				}
				candidate = append(candidate, level[j].info)
				totalSize += size
			}

			if len(candidate) >= p.mergeFactor {
				// Check max merge docs
				totalDocs := 0
				for _, seg := range candidate {
					totalDocs += seg.SegmentInfo().DocCount()
				}
				if totalDocs <= p.maxMergeDocs {
					merge := NewOneMerge(candidate)
					spec.Add(merge)
					i += len(candidate)
					continue
				}
			}
			i++
		}
	}

	if spec.Size() == 0 {
		return nil, nil
	}

	return spec, nil
}

// getLevelSizes groups segments by level based on their sizes.
func (p *LogMergePolicy) getLevelSizes(segments []logSegInfo) [][]logSegInfo {
	if len(segments) == 0 {
		return nil
	}

	// Group segments into levels based on size
	levels := make([][]logSegInfo, 0)
	currentLevel := make([]logSegInfo, 0)

	for _, seg := range segments {
		if len(currentLevel) == 0 {
			currentLevel = append(currentLevel, seg)
		} else {
			// Check if this segment belongs in the current level
			// Segments within a factor of levelSize of each other are in the same level
			firstSize := currentLevel[0].size
			if firstSize == 0 {
				firstSize = 1
			}
			segSize := seg.size
			if segSize == 0 {
				segSize = 1
			}

			// Check if sizes are within a factor of levelSize
			if segSize >= firstSize && segSize/firstSize <= p.levelSize {
				currentLevel = append(currentLevel, seg)
			} else {
				levels = append(levels, currentLevel)
				currentLevel = []logSegInfo{seg}
			}
		}
	}

	if len(currentLevel) > 0 {
		levels = append(levels, currentLevel)
	}

	return levels
}

// FindForcedMerges finds forced merges to reduce segment count.
// This implements GC-636: LogMergePolicy.FindForcedMerges
func (p *LogMergePolicy) FindForcedMerges(
	infos *SegmentInfos,
	maxSegmentCount int,
	segmentsToMerge map[*SegmentCommitInfo]bool,
	mergeContext MergeContext,
) (*MergeSpecification, error) {

	if mergeContext == nil {
		return nil, nil
	}

	merging := mergeContext.GetMergingSegments()

	// Collect eligible segments
	type segInfo struct {
		info *SegmentCommitInfo
		size int64
	}
	eligible := make([]segInfo, 0)

	for sci := range infos.Iterator() {
		_, shouldMerge := segmentsToMerge[sci]
		if !shouldMerge {
			continue
		}
		if merging[sci] {
			continue
		}

		size := p.Size(sci, mergeContext)
		// Skip segments that are too large for forced merge
		if p.maxMergeSizeForForcedMerge > 0 && size > p.maxMergeSizeForForcedMerge {
			continue
		}

		eligible = append(eligible, segInfo{info: sci, size: size})
	}

	if len(eligible) <= maxSegmentCount {
		return nil, nil
	}

	// Sort by size (smallest first)
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].size < eligible[j].size
	})

	spec := NewMergeSpecification()

	// Merge from smallest to largest until we reach maxSegmentCount
	for len(eligible) > maxSegmentCount {
		// Find mergeFactor segments to merge
		candidate := make([]*SegmentCommitInfo, 0, p.mergeFactor)
		var totalSize int64
		var totalDocs int

		for i := 0; i < len(eligible) && len(candidate) < p.mergeFactor; i++ {
			size := eligible[i].size
			docs := eligible[i].info.SegmentInfo().DocCount()

			if (totalSize+size > p.maxMergeSize || totalDocs+docs > p.maxMergeDocs) && len(candidate) >= 2 {
				break
			}

			candidate = append(candidate, eligible[i].info)
			totalSize += size
			totalDocs += docs
		}

		if len(candidate) < 2 {
			// Can't merge any more
			break
		}

		merge := NewOneMerge(candidate)
		spec.Add(merge)

		// Remove merged segments from eligible
		merged := make(map[*SegmentCommitInfo]bool)
		for _, seg := range candidate {
			merged[seg] = true
		}

		newEligible := make([]segInfo, 0, len(eligible)-len(candidate)+1)
		for _, seg := range eligible {
			if !merged[seg.info] {
				newEligible = append(newEligible, seg)
			}
		}
		eligible = newEligible
	}

	if spec.Size() == 0 {
		return nil, nil
	}

	return spec, nil
}

// FindForcedDeletesMerges finds merges to expunge deleted documents.
// This implements GC-636: LogMergePolicy.FindForcedDeletesMerges
func (p *LogMergePolicy) FindForcedDeletesMerges(
	infos *SegmentInfos,
	mergeContext MergeContext,
) (*MergeSpecification, error) {

	if mergeContext == nil {
		return nil, nil
	}

	merging := mergeContext.GetMergingSegments()

	// Collect segments with deletes
	type segInfo struct {
		info     *SegmentCommitInfo
		size     int64
		delCount int
	}
	eligible := make([]segInfo, 0)

	for sci := range infos.Iterator() {
		if merging[sci] {
			continue
		}

		delCount := mergeContext.NumDeletesToMerge(sci)
		if delCount > 0 {
			size := p.Size(sci, mergeContext)
			eligible = append(eligible, segInfo{info: sci, size: size, delCount: delCount})
		}
	}

	if len(eligible) == 0 {
		return nil, nil
	}

	// Sort by delete percentage (highest first)
	sort.Slice(eligible, func(i, j int) bool {
		iPct := float64(eligible[i].delCount) / float64(eligible[i].info.SegmentInfo().DocCount())
		jPct := float64(eligible[j].delCount) / float64(eligible[j].info.SegmentInfo().DocCount())
		return iPct > jPct
	})

	spec := NewMergeSpecification()

	// Merge segments with high delete percentages
	for i := 0; i+p.mergeFactor <= len(eligible); i += p.mergeFactor {
		candidate := make([]*SegmentCommitInfo, 0, p.mergeFactor)
		var totalSize int64
		var totalDocs int

		for j := i; j < len(eligible) && len(candidate) < p.mergeFactor; j++ {
			size := eligible[j].size
			docs := eligible[j].info.SegmentInfo().DocCount()

			if (totalSize+size > p.maxMergeSize || totalDocs+docs > p.maxMergeDocs) && len(candidate) >= 2 {
				break
			}

			candidate = append(candidate, eligible[j].info)
			totalSize += size
			totalDocs += docs
		}

		if len(candidate) >= 2 {
			merge := NewOneMerge(candidate)
			spec.Add(merge)
		}
	}

	if spec.Size() == 0 {
		return nil, nil
	}

	return spec, nil
}

// UseCompoundFile returns true if the merged segment should use compound file.
func (p *LogMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
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
func (p *LogMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return delCount
}

// KeepFullyDeletedSegment returns false by default.
func (p *LogMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return false
}

// String returns a string representation of the policy.
func (p *LogMergePolicy) String() string {
	return fmt.Sprintf("[LogMergePolicy: minMergeMB=%.1f, maxMergeMB=%.1f, mergeFactor=%d, maxMergeDocs=%d]",
		p.GetMinMergeMB(), p.GetMaxMergeMB(), p.mergeFactor, p.maxMergeDocs)
}

// LogByteSizeMergePolicy merges segments based on their byte size.
// This is the Go port of Lucene's org.apache.lucene.index.LogByteSizeMergePolicy.
//
// This implements GC-637: LogByteSizeMergePolicy
type LogByteSizeMergePolicy struct {
	*LogMergePolicy
}

// NewLogByteSizeMergePolicy creates a new LogByteSizeMergePolicy.
// This implements GC-637: LogByteSizeMergePolicy constructor
func NewLogByteSizeMergePolicy() *LogByteSizeMergePolicy {
	return &LogByteSizeMergePolicy{
		LogMergePolicy: NewLogMergePolicy(),
	}
}

// String returns a string representation of the policy.
func (p *LogByteSizeMergePolicy) String() string {
	return fmt.Sprintf("[LogByteSizeMergePolicy: minMergeMB=%.1f, maxMergeMB=%.1f, mergeFactor=%d, maxMergeDocs=%d]",
		p.GetMinMergeMB(), p.GetMaxMergeMB(), p.GetMergeFactor(), p.GetMaxMergeDocs())
}

// LogDocMergePolicy merges segments based on their document count.
// This is the Go port of Lucene's org.apache.lucene.index.LogDocMergePolicy.
//
// This implements GC-638: LogDocMergePolicy
type LogDocMergePolicy struct {
	*LogMergePolicy
}

// NewLogDocMergePolicy creates a new LogDocMergePolicy.
// This implements GC-638: LogDocMergePolicy constructor
func NewLogDocMergePolicy() *LogDocMergePolicy {
	return &LogDocMergePolicy{
		LogMergePolicy: NewLogMergePolicy(),
	}
}

// Size returns the size of a segment based on document count.
// This overrides the byte size calculation to use document count instead.
func (p *LogDocMergePolicy) Size(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	docCount := int64(info.SegmentInfo().DocCount())

	if p.GetCalibrateSizeByDeletes() && mergeContext != nil {
		delCount := mergeContext.NumDeletesToMerge(info)
		if docCount > 0 && delCount > 0 {
			delRatio := float64(delCount) / float64(docCount)
			if delRatio > 1.0 {
				delRatio = 1.0
			}
			docCount = int64(float64(docCount) * (1.0 - delRatio))
		}
	}

	return docCount
}

// String returns a string representation of the policy.
func (p *LogDocMergePolicy) String() string {
	return fmt.Sprintf("[LogDocMergePolicy: mergeFactor=%d, maxMergeDocs=%d]",
		p.GetMergeFactor(), p.GetMaxMergeDocs())
}

// Ensure interfaces are implemented
var _ MergePolicy = (*LogMergePolicy)(nil)
var _ MergePolicy = (*LogByteSizeMergePolicy)(nil)
var _ MergePolicy = (*LogDocMergePolicy)(nil)

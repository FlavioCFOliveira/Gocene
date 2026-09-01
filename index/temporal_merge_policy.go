// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TemporalMergePolicy is a merge policy that groups segments by time windows and merges segments within the same window.
// This policy is designed for time-series data where documents contain a timestamp field indexed as a LongPoint.
//
// Mirrors org.apache.lucene.index.TemporalMergePolicy from Apache Lucene 10.5.0.
type TemporalMergePolicy struct {
	*BaseMergePolicy

	temporalField              string
	baseTimeSeconds            int64
	minThreshold               int
	useExponentialBuckets      bool
	maxWindowSizeSeconds       int64
	maxAgeSeconds              int64
	maxThreshold               int
	compactionRatio            float64
	forceMergeDeletesPctAllowed float64

	// segmentDateRangeOverrides allows providing date ranges for segments manually (mostly for testing).
	segmentDateRangeOverrides map[*SegmentCommitInfo]*segmentDateRange
}

type segmentDateRange struct {
	minDate int64
	maxDate int64
}

// NewTemporalMergePolicy constructs a TemporalMergePolicy with default settings.
func NewTemporalMergePolicy() *TemporalMergePolicy {
	return &TemporalMergePolicy{
		BaseMergePolicy: NewBaseMergePolicy(),
		temporalField:    "",
		baseTimeSeconds:  3600,
		minThreshold:     4,
		useExponentialBuckets: true,
		maxWindowSizeSeconds:  int64(24 * 60 * 60 * 365),
		maxAgeSeconds:        math.MaxInt64,
		maxThreshold:        8,
		compactionRatio:      1.2,
		forceMergeDeletesPctAllowed: 10.0,
	}
}

func (p *TemporalMergePolicy) SetTemporalField(temporalField string) *TemporalMergePolicy {
	if temporalField == "" {
		panic("temporalField cannot be blank")
	}
	p.temporalField = temporalField
	return p
}

func (p *TemporalMergePolicy) GetTemporalField() string {
	return p.temporalField
}

func (p *TemporalMergePolicy) SetBaseTimeInSeconds(baseTimeInSeconds int64) *TemporalMergePolicy {
	if baseTimeInSeconds <= 0 {
		panic("baseTimeSeconds must be positive")
	}
	p.baseTimeSeconds = baseTimeInSeconds
	return p
}

func (p *TemporalMergePolicy) GetBaseTimeInSeconds() int64 {
	return p.baseTimeSeconds
}

func (p *TemporalMergePolicy) SetMinThreshold(minThreshold int) *TemporalMergePolicy {
	if minThreshold < 2 {
		panic("minThreshold must be at least 2")
	}
	if minThreshold > p.maxThreshold {
		panic(fmt.Sprintf("minThreshold cannot exceed maxThreshold (%d)", p.maxThreshold))
	}
	p.minThreshold = minThreshold
	return p
}

func (p *TemporalMergePolicy) GetMinThreshold() int {
	return p.minThreshold
}

func (p *TemporalMergePolicy) DisableExponentialBuckets() *TemporalMergePolicy {
	p.useExponentialBuckets = false
	return p
}

func (p *TemporalMergePolicy) GetUseExponentialBuckets() bool {
	return p.useExponentialBuckets
}

func (p *TemporalMergePolicy) SetMaxThreshold(maxThreshold int) *TemporalMergePolicy {
	if maxThreshold < p.minThreshold {
		panic(fmt.Sprintf("maxThreshold must be >= minThreshold (%d)", p.minThreshold))
	}
	p.maxThreshold = maxThreshold
	return p
}

func (p *TemporalMergePolicy) GetMaxThreshold() int {
	return p.maxThreshold
}

func (p *TemporalMergePolicy) SetCompactionRatio(compactionRatio float64) *TemporalMergePolicy {
	if compactionRatio < 1.0 {
		panic("compactionRatio must be >= 1.0")
	}
	p.compactionRatio = compactionRatio
	return p
}

func (p *TemporalMergePolicy) GetCompactionRatio() float64 {
	return p.compactionRatio
}

func (p *TemporalMergePolicy) SetMaxWindowSizeSeconds(maxWindowSizeSeconds int64) *TemporalMergePolicy {
	if maxWindowSizeSeconds <= 0 {
		panic("maxWindowSizeSeconds must be positive")
	}
	p.maxWindowSizeSeconds = maxWindowSizeSeconds
	return p
}

func (p *TemporalMergePolicy) GetMaxWindowSizeSeconds() int64 {
	return p.maxWindowSizeSeconds
}

func (p *TemporalMergePolicy) SetMaxAgeSeconds(maxAgeSeconds int64) *TemporalMergePolicy {
	if maxAgeSeconds <= 0 {
		panic("maxAgeSeconds must be positive")
	}
	p.maxAgeSeconds = maxAgeSeconds
	return p
}

func (p *TemporalMergePolicy) GetMaxAgeSeconds() int64 {
	return p.maxAgeSeconds
}

func (p *TemporalMergePolicy) SetForceMergeDeletesPctAllowed(pct float64) *TemporalMergePolicy {
	if pct < 0.0 {
		panic("forceMergeDeletesPctAllowed must be >= 0")
	}
	p.forceMergeDeletesPctAllowed = pct
	return p
}

func (p *TemporalMergePolicy) GetForceMergeDeletesPctAllowed() float64 {
	return p.forceMergeDeletesPctAllowed
}

func (p *TemporalMergePolicy) FindMerges(trigger MergeTrigger, segments *SegmentInfos, context MergeContext) (*MergeSpecification, error) {
	if p.temporalField == "" || segments.Size() == 0 {
		return nil, nil
	}

	alreadyMerging := context.GetMergingSegments()
	allRanges := p.resolveSegmentDateRanges(segments)

	if len(allRanges) == 0 {
		return nil, nil
	}

	windowBuckets := make(map[int64][]*SegmentCommitInfo)
	now := time.Now().UnixMilli()

	for sci := range segments.Iterator() {
		if alreadyMerging[sci] {
			continue
		}

		rangeInfo, ok := allRanges[sci]
		if ok {
			p.assignToBucket(windowBuckets, now, sci, rangeInfo)
		}
	}

	if len(windowBuckets) == 0 {
		return nil, nil
	}

	return p.findMergeCandidates(windowBuckets, allRanges)
}

func (p *TemporalMergePolicy) FindForcedMerges(segmentInfos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	if maxSegmentCount < 1 {
		return nil, fmt.Errorf("maxSegmentCount must be >= 1")
	}

	segmentDateRanges := p.resolveSegmentDateRanges(segmentInfos)
	if len(segmentDateRanges) == 0 {
		return nil, nil
	}

	mergeAll := segmentsToMerge == nil || len(segmentsToMerge) == 0
	merging := mergeContext.GetMergingSegments()

	eligibleRanges := make(map[*SegmentCommitInfo]*segmentDateRange)
	forceMergeRunning := false
	for sci := range segmentInfos.Iterator() {
		include := mergeAll || segmentsToMerge[sci]
		if include {
			if merging[sci] {
				forceMergeRunning = true
				break
			}

			rangeInfo, ok := segmentDateRanges[sci]
			if ok {
				eligibleRanges[sci] = rangeInfo
			}
		}
	}

	if forceMergeRunning {
		return nil, nil
	}

	if len(eligibleRanges) == 0 {
		return nil, nil
	}

	windowBuckets := p.groupSegmentsByTimeWindow(eligibleRanges)
	if len(windowBuckets) == 0 {
		return nil, nil
	}

	eligibleWindows := len(windowBuckets)
	segmentsPerWindow := maxSegmentCount / eligibleWindows
	if segmentsPerWindow < 1 {
		segmentsPerWindow = 1
	}
	if segmentsPerWindow > p.maxThreshold {
		segmentsPerWindow = p.maxThreshold
	}

	var spec *MergeSpecification
	for windowStart, bucketSegments := range windowBuckets {
		if windowStart == -1 {
			continue
		}

		if len(bucketSegments) < 2 {
			continue
		}

		bucketSpec := p.buildForcedMerges(bucketSegments, segmentsPerWindow)
		if bucketSpec == nil {
			continue
		}

		if spec == nil {
			spec = NewMergeSpecification()
		}
		for _, merge := range bucketSpec.Merges {
			spec.Add(merge)
		}
	}

	return spec, nil
}

func (p *TemporalMergePolicy) FindForcedDeletesMerges(segmentInfos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	if mergeContext == nil {
		return nil, nil
	}

	segmentDateRanges := p.resolveSegmentDateRanges(segmentInfos)
	if len(segmentDateRanges) == 0 {
		return nil, nil
	}

	merging := mergeContext.GetMergingSegments()
	candidateRanges := make(map[*SegmentCommitInfo]*segmentDateRange)
	deleteRatios := make(map[*SegmentCommitInfo]float64)

	for sci := range segmentInfos.Iterator() {
		if merging[sci] {
			continue
		}

		delCount := mergeContext.NumDeletesToMerge(sci, 0)
		if delCount <= 0 {
			continue
		}
		maxDoc := sci.Info.MaxDoc()
		if maxDoc < 1 {
			maxDoc = 1
		}
		pctDeletes := (100.0 * float64(delCount)) / float64(maxDoc)
		if pctDeletes > p.forceMergeDeletesPctAllowed {
			rangeInfo, ok := segmentDateRanges[sci]
			if ok {
				candidateRanges[sci] = rangeInfo
				deleteRatios[sci] = pctDeletes
			}
		}
	}

	if len(candidateRanges) == 0 {
		return nil, nil
	}

	windowBuckets := p.groupSegmentsByTimeWindow(candidateRanges)
	var spec *MergeSpecification

	for _, bucketSegments := range windowBuckets {
		if len(bucketSegments) < 2 {
			continue
		}

		sort.Slice(bucketSegments, func(i, j int) bool {
			return deleteRatios[bucketSegments[j]] > deleteRatios[bucketSegments[i]]
		})

		bucketSpec := p.buildSequentialMerges(bucketSegments)
		if bucketSpec == nil {
			continue
		}

		if spec == nil {
			spec = NewMergeSpecification()
		}
		for _, merge := range bucketSpec.Merges {
			spec.Add(merge)
		}
	}

	return spec, nil
}

func (p *TemporalMergePolicy) resolveSegmentDateRanges(segments *SegmentInfos) map[*SegmentCommitInfo]*segmentDateRange {
	if p.segmentDateRangeOverrides != nil {
		return p.segmentDateRangeOverrides
	}
	return p.extractSegmentDateRanges(segments)
}

func (p *TemporalMergePolicy) extractSegmentDateRanges(segments *SegmentInfos) map[*SegmentCommitInfo]*segmentDateRange {
	ranges := make(map[*SegmentCommitInfo]*segmentDateRange)

	for sci := range segments.Iterator() {
		rangeInfo, err := p.extractDateRangeFromSegment(sci)
		if err != nil {
			// In a real production system we would use a logger here
			continue
		}
		if rangeInfo != nil {
			ranges[sci] = rangeInfo
		}
	}
	return ranges
}

func (p *TemporalMergePolicy) extractDateRangeFromSegment(sci *SegmentCommitInfo) (*segmentDateRange, error) {
	si := sci.Info
	var compoundDir util.Directory
	var readerDir util.Directory

	if si.GetUseCompoundFile() {
		var err error
		compoundDir, err = si.GetCodec().CompoundFormat().GetCompoundReader(si.Dir, si)
		if err != nil {
			return nil, err
		}
		readerDir = compoundDir
	} else {
		readerDir = si.Dir
	}

	defer func() {
		if compoundDir != nil {
			compoundDir.Close()
		}
	}()

	fieldInfos := si.GetCodec().FieldInfosFormat().Read(readerDir, si, "", spi.IOContextReadOnce)
	fieldInfo := fieldInfos.FieldInfo(p.temporalField)
	if fieldInfo == nil {
		return nil, nil
	}

	if fieldInfo.GetPointDimensionCount() == 0 {
		return nil, nil
	}

	pointsFormat := si.GetCodec().PointsFormat()
	pointsReader, err := pointsFormat.FieldsReader(readerDir, si, fieldInfos)
	if err != nil {
		return nil, err
	}
	defer pointsReader.Close()

	pointValues := pointsReader.GetValues(p.temporalField)
	if pointValues == nil {
		return nil, nil
	}

	minPacked := pointValues.GetMinPackedValue()
	maxPacked := pointValues.GetMaxPackedValue()
	if minPacked == nil || maxPacked == nil {
		return nil, nil
	}

	// LongPoint.decodeDimension logic
	minDate := spi.DecodeDimension(minPacked, 0)
	maxDate := spi.DecodeDimension(maxPacked, 0)

	divisor := p.getTemporalFieldDivisor(maxDate)
	var minDateMillis, maxDateMillis int64
	if divisor < 0 {
		multiplier := -divisor
		minDateMillis = minDate * multiplier
		maxDateMillis = maxDate * multiplier
	} else {
		minDateMillis = minDate / divisor
		maxDateMillis = maxDate / divisor
	}

	return &segmentDateRange{minDate: minDateMillis, maxDate: maxDateMillis}, nil
}

func (p *TemporalMergePolicy) getTemporalFieldDivisor(maxValue int64) int64 {
	if maxValue > 100_000_000_000_000 {
		return 1000
	} else if maxValue > 100_000_000_000 {
		return 1
	} else {
		return -1000
	}
}

func (p *TemporalMergePolicy) groupSegmentsByTimeWindow(segmentDateRanges map[*SegmentCommitInfo]*segmentDateRange) map[int64][]*SegmentCommitInfo {
	buckets := make(map[int64][]*SegmentCommitInfo)
	now := time.Now().UnixMilli()

	for sci, rangeInfo := range segmentDateRanges {
		p.assignToBucket(buckets, now, sci, rangeInfo)
	}
	return buckets
}

func (p *TemporalMergePolicy) assignToBucket(buckets map[int64][]*SegmentCommitInfo, now int64, sci *SegmentCommitInfo, rangeInfo *segmentDateRange) {
	maxDateSeconds := rangeInfo.maxDate / 1000
	bucket := p.getBucketForTimestamp(maxDateSeconds, now/1000)
	buckets[bucket] = append(buckets[bucket], sci)
}

func (p *TemporalMergePolicy) getBucketForTimestamp(timestampSeconds, nowSeconds int64) int64 {
	ageSeconds := nowSeconds - timestampSeconds
	if ageSeconds < 0 {
		ageSeconds = 0
	}

	if ageSeconds > p.maxAgeSeconds {
		return -1
	}

	if !p.useExponentialBuckets {
		return (timestampSeconds / p.baseTimeSeconds) * p.baseTimeSeconds
	}

	bucketSizeSeconds := p.baseTimeSeconds
	for ageSeconds >= bucketSizeSeconds*int64(p.minThreshold) && bucketSizeSeconds < p.maxWindowSizeSeconds {
		bucketSizeSeconds *= int64(p.minThreshold)
	}

	if bucketSizeSeconds > p.maxWindowSizeSeconds {
		bucketSizeSeconds = p.maxWindowSizeSeconds
	}

	return (timestampSeconds / bucketSizeSeconds) * bucketSizeSeconds
}

func (p *TemporalMergePolicy) findMergeCandidates(buckets map[int64][]*SegmentCommitInfo, segmentDateRanges map[*SegmentCommitInfo]*segmentDateRange) (*MergeSpecification, error) {
	var spec *MergeSpecification

	// TreeMap in Java is sorted by key. In Go we need to sort the keys.
	var windowStarts []int64
	for k := range buckets {
		windowStarts = append(windowStarts, k)
	}
	sort.Slice(windowStarts, func(i, j int) bool { return windowStarts[i] < windowStarts[j] })

	for _, windowStart := range windowStarts {
		segmentsInWindow := buckets[windowStart]

		if windowStart == -1 {
			continue
		}

		if len(segmentsInWindow) < p.minThreshold {
			continue
		}

		mergesForWindow := p.planWindowMerges(windowStart, segmentsInWindow, segmentDateRanges)
		if len(mergesForWindow) == 0 {
			continue
		}

		if spec == nil {
			spec = NewMergeSpecification()
		}

		for _, mergeSegments := range mergesForWindow {
			spec.Add(NewOneMerge(mergeSegments))
		}
	}

	return spec, nil
}

func (p *TemporalMergePolicy) planWindowMerges(windowStart int64, segmentsInWindow []*SegmentCommitInfo, segmentDateRanges map[*SegmentCommitInfo]*segmentDateRange) [][]*SegmentCommitInfo {
	ordered := make([]*SegmentCommitInfo, len(segmentsInWindow))
	copy(ordered, segmentsInWindow)
	sort.Slice(ordered, func(i, j int) bool {
		return segmentDateRanges[ordered[i]].maxDate > segmentDateRanges[ordered[j]].maxDate
	})

	var planned [][]*SegmentCommitInfo
	cursor := 0

	for len(ordered)-cursor >= p.minThreshold {
		var totalDocs int64
		var largestDocs int64
		end := cursor
		emittedMerge := false

		for end < len(ordered) && end-cursor < p.maxThreshold {
			candidate := ordered[end]
			docCount := int64(candidate.Info.MaxDoc())
			totalDocs += docCount
			if docCount > largestDocs {
				largestDocs = docCount
			}
			end++

			candidateSize := end - cursor
			if candidateSize < p.minThreshold {
				continue
			}

			reachedMax := candidateSize == p.maxThreshold
			exhaustedSegments := end == len(ordered)

			if p.compactionRatio <= 1.0 {
				if reachedMax || exhaustedSegments {
					planned = append(planned, ordered[cursor:end])
					cursor = end
					emittedMerge = true
					break
				}
			} else {
				ratioSatisfied := float64(totalDocs) >= math.Ceil(float64(largestDocs)*p.compactionRatio)
				if ratioSatisfied || reachedMax {
					planned = append(planned, ordered[cursor:end])
					cursor = end
					emittedMerge = true
					break
				}
			}
		}

		if !emittedMerge {
			break
		}
	}

	return planned
}

func (p *TemporalMergePolicy) buildForcedMerges(candidates []*SegmentCommitInfo, maxSegmentCount int) *MergeSpecification {
	if len(candidates) < 2 {
		return nil
	}

	var spec *MergeSpecification
	remaining := len(candidates)
	offset := 0

	for remaining > maxSegmentCount && (len(candidates)-offset) >= 2 {
		neededReduction := remaining - maxSegmentCount
		mergeInputs := maxThreshold(2, minInt(p.maxThreshold, neededReduction+1))
		mergeInputs = minInt(mergeInputs, len(candidates)-offset)

		batch := candidates[offset : offset+mergeInputs]

		if spec == nil {
			spec = NewMergeSpecification()
		}
		spec.Add(NewOneMerge(append([]*SegmentCommitInfo(nil), batch...)))
		offset += mergeInputs
		remaining -= (mergeInputs - 1)
	}

	return spec
}

func (p *TemporalMergePolicy) buildSequentialMerges(candidates []*SegmentCommitInfo) *MergeSpecification {
	if len(candidates) < 2 {
		return nil
	}

	var spec *MergeSpecification
	batch := make([]*SegmentCommitInfo, 0, p.maxThreshold)

	for _, info := range candidates {
		batch = append(batch, info)
		if len(batch) == p.maxThreshold {
			if spec == nil {
				spec = NewMergeSpecification()
			}
			spec.Add(NewOneMerge(append([]*SegmentCommitInfo(nil), batch...)))
			batch = batch[:0]
		}
	}

	if len(batch) > 1 {
		if spec == nil {
			spec = NewMergeSpecification()
		}
		spec.Add(NewOneMerge(append([]*SegmentCommitInfo(nil), batch...)))
	}

	return spec
}

func (p *TemporalMergePolicy) String() string {
	return fmt.Sprintf("[TemporalMergePolicy: temporalField=%s, baseTimeSeconds=%d, minThreshold=%d, maxThreshold=%d, useExponentialBuckets=%v, maxWindowSizeSeconds=%d, maxAgeSeconds=%d, compactionRatio=%.1f, forceMergeDeletesPctAllowed=%.1f]",
		p.temporalField, p.baseTimeSeconds, p.minThreshold, p.maxThreshold, p.useExponentialBuckets, p.maxWindowSizeSeconds, p.maxAgeSeconds, p.compactionRatio, p.forceMergeDeletesPctAllowed)
}

func maxThreshold(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

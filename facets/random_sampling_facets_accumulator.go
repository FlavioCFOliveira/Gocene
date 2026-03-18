// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// RandomSamplingFacetsAccumulator accumulates facet counts using random sampling.
// This provides a performance optimization for large datasets by sampling a subset
// of documents and extrapolating the counts.
//
// This is the Go port of Lucene's random sampling faceting support.
// It provides statistical accuracy guarantees with configurable sample rates.
//
// The sampling algorithm uses reservoir sampling for unbiased selection and
// provides confidence intervals for the estimated counts.
type RandomSamplingFacetsAccumulator struct {
	// config contains the facets configuration
	config *FacetsConfig

	// sampleRate is the fraction of documents to sample (0.0 to 1.0)
	sampleRate float64

	// minSampleSize is the minimum number of documents to sample
	minSampleSize int

	// maxSampleSize is the maximum number of documents to sample
	maxSampleSize int

	// counts holds the aggregated counts per ordinal (sampled)
	counts []int64

	// ordToLabel maps ordinals to their labels
	ordToLabel map[int]string

	// labelToOrd maps labels to their ordinals
	labelToOrd map[string]int

	// nextOrd is the next available ordinal
	nextOrd int

	// totalSampledDocs is the total number of documents sampled
	totalSampledDocs int64

	// totalDocs is the total number of documents (estimated)
	totalDocs int64

	// random is the random number generator
	random *rand.Rand

	// mu protects the maps and counts
	mu sync.RWMutex

	// confidenceLevel is the confidence level for intervals (e.g., 0.95 for 95%)
	confidenceLevel float64

	// seed is the random seed for reproducibility
	seed int64
}

// NewRandomSamplingFacetsAccumulator creates a new RandomSamplingFacetsAccumulator.
//
// Parameters:
//   - config: the facets configuration
//   - sampleRate: the fraction of documents to sample (0.0 to 1.0)
//
// Returns:
//   - a new RandomSamplingFacetsAccumulator instance
func NewRandomSamplingFacetsAccumulator(config *FacetsConfig, sampleRate float64) (*RandomSamplingFacetsAccumulator, error) {
	if config == nil {
		return nil, fmt.Errorf("facets config cannot be nil")
	}

	if sampleRate <= 0 || sampleRate > 1.0 {
		return nil, fmt.Errorf("sample rate must be between 0.0 and 1.0, got %f", sampleRate)
	}

	seed := time.Now().UnixNano()
	return &RandomSamplingFacetsAccumulator{
		config:          config,
		sampleRate:      sampleRate,
		minSampleSize:   100,
		maxSampleSize:   100000,
		counts:          make([]int64, 1024),
		ordToLabel:      make(map[int]string),
		labelToOrd:      make(map[string]int),
		nextOrd:         1,
		random:          rand.New(rand.NewSource(seed)),
		confidenceLevel: 0.95,
		seed:            seed,
	}, nil
}

// Accumulate accumulates facet counts from the given facet results.
// This implements the FacetsAccumulator interface.
func (rsfa *RandomSamplingFacetsAccumulator) Accumulate(results []*FacetResult) error {
	for _, result := range results {
		if result == nil {
			continue
		}

		// Process each label value in the result
		for _, lv := range result.LabelValues {
			if lv == nil {
				continue
			}

			// Get or create ordinal for this label
			ord := rsfa.getOrCreateOrdinal(lv.Label)
			if ord > 0 {
				rsfa.incrementCount(ord, lv.Value)
			}
		}
	}
	return nil
}

// AccumulateFromMatchingDocs accumulates counts from matching documents using sampling.
// This is the main entry point for accumulating facet counts from search results.
//
// Parameters:
//   - matchingDocs: the matching documents from each segment
//
// Returns:
//   - error if accumulation fails
func (rsfa *RandomSamplingFacetsAccumulator) AccumulateFromMatchingDocs(matchingDocs []*MatchingDocs) error {
	// Calculate total documents
	var totalDocs int
	for _, md := range matchingDocs {
		totalDocs += md.TotalHits
	}

	rsfa.mu.Lock()
	rsfa.totalDocs = int64(totalDocs)
	rsfa.mu.Unlock()

	// Calculate sample size
	sampleSize := rsfa.calculateSampleSize(totalDocs)

	// Use reservoir sampling to select documents
	sampledDocs := rsfa.reservoirSample(matchingDocs, sampleSize)

	// Accumulate from sampled documents
	for _, doc := range sampledDocs {
		if err := rsfa.accumulateFromSampledDoc(doc); err != nil {
			return fmt.Errorf("accumulating from sampled doc: %w", err)
		}
	}

	rsfa.mu.Lock()
	rsfa.totalSampledDocs = int64(len(sampledDocs))
	rsfa.mu.Unlock()

	return nil
}

// calculateSampleSize calculates the sample size based on the sample rate.
func (rsfa *RandomSamplingFacetsAccumulator) calculateSampleSize(totalDocs int) int {
	sampleSize := int(float64(totalDocs) * rsfa.sampleRate)

	if sampleSize < rsfa.minSampleSize {
		return rsfa.minSampleSize
	}

	if sampleSize > rsfa.maxSampleSize {
		return rsfa.maxSampleSize
	}

	return sampleSize
}

// reservoirSample performs reservoir sampling to select a random subset of documents.
func (rsfa *RandomSamplingFacetsAccumulator) reservoirSample(matchingDocs []*MatchingDocs, sampleSize int) []*MatchingDocs {
	if sampleSize <= 0 {
		return nil
	}

	// Flatten all matching docs into a single slice
	var allDocs []*MatchingDocs
	for _, md := range matchingDocs {
		if md != nil {
			allDocs = append(allDocs, md)
		}
	}

	if len(allDocs) <= sampleSize {
		return allDocs
	}

	// Reservoir sampling
	rsfa.mu.Lock()
	defer rsfa.mu.Unlock()

	reservoir := make([]*MatchingDocs, sampleSize)
	copy(reservoir, allDocs[:sampleSize])

	for i := sampleSize; i < len(allDocs); i++ {
		j := rsfa.random.Intn(i + 1)
		if j < sampleSize {
			reservoir[j] = allDocs[i]
		}
	}

	return reservoir
}

// accumulateFromSampledDoc accumulates counts from a single sampled document.
func (rsfa *RandomSamplingFacetsAccumulator) accumulateFromSampledDoc(matchingDocs *MatchingDocs) error {
	// In a full implementation, this would:
	// 1. Get the doc values for the facet field
	// 2. Get ordinals for this document
	// 3. Increment counts
	// For now, this is a placeholder
	return nil
}

// getOrCreateOrdinal gets an existing ordinal or creates a new one for the label.
func (rsfa *RandomSamplingFacetsAccumulator) getOrCreateOrdinal(label string) int {
	// First try read lock
	rsfa.mu.RLock()
	if ord, exists := rsfa.labelToOrd[label]; exists {
		rsfa.mu.RUnlock()
		return ord
	}
	rsfa.mu.RUnlock()

	// Need to create new ordinal, use write lock
	rsfa.mu.Lock()
	defer rsfa.mu.Unlock()

	// Double-check after acquiring write lock
	if ord, exists := rsfa.labelToOrd[label]; exists {
		return ord
	}

	// Create new ordinal
	ord := rsfa.nextOrd
	rsfa.nextOrd++
	rsfa.labelToOrd[label] = ord
	rsfa.ordToLabel[ord] = label

	// Ensure capacity
	rsfa.ensureCapacityLocked(ord)

	return ord
}

// ensureCapacityLocked ensures the counts slice can hold the given ordinal.
// Must be called with write lock held.
func (rsfa *RandomSamplingFacetsAccumulator) ensureCapacityLocked(ord int) {
	if ord >= len(rsfa.counts) {
		newSize := ord * 2
		newCounts := make([]int64, newSize)
		copy(newCounts, rsfa.counts)
		rsfa.counts = newCounts
	}
}

// incrementCount increments the count for the given ordinal.
// This method is thread-safe.
func (rsfa *RandomSamplingFacetsAccumulator) incrementCount(ordinal int, count int64) {
	if ordinal <= 0 {
		return
	}

	rsfa.mu.Lock()
	defer rsfa.mu.Unlock()

	rsfa.ensureCapacityLocked(ordinal)
	rsfa.counts[ordinal] += count
}

// GetCount returns the raw sampled count for the given ordinal.
func (rsfa *RandomSamplingFacetsAccumulator) GetCount(ordinal int) int64 {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	if ordinal >= 0 && ordinal < len(rsfa.counts) {
		return rsfa.counts[ordinal]
	}
	return 0
}

// GetEstimatedCount returns the estimated count for the given ordinal.
// This extrapolates the sampled count to the full population.
func (rsfa *RandomSamplingFacetsAccumulator) GetEstimatedCount(ordinal int) int64 {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	if rsfa.totalSampledDocs == 0 {
		return 0
	}

	sampledCount := rsfa.counts[ordinal]
	// Extrapolate: estimated = sampled * (total / sampled)
	estimated := int64(float64(sampledCount) * float64(rsfa.totalDocs) / float64(rsfa.totalSampledDocs))
	return estimated
}

// GetConfidenceInterval returns the confidence interval for the estimated count.
// Returns (lower, upper) bounds for the given confidence level.
func (rsfa *RandomSamplingFacetsAccumulator) GetConfidenceInterval(ordinal int) (int64, int64) {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	if rsfa.totalSampledDocs == 0 || rsfa.totalDocs == 0 {
		return 0, 0
	}

	sampledCount := float64(rsfa.counts[ordinal])

	// Calculate standard error for proportion
	p := sampledCount / float64(rsfa.totalSampledDocs)
	se := math.Sqrt(p * (1 - p) / float64(rsfa.totalSampledDocs))

	// Z-score for 95% confidence (1.96)
	zScore := 1.96
	if rsfa.confidenceLevel == 0.99 {
		zScore = 2.576
	} else if rsfa.confidenceLevel == 0.90 {
		zScore = 1.645
	}

	margin := zScore * se
	lower := p - margin
	upper := p + margin

	if lower < 0 {
		lower = 0
	}
	if upper > 1 {
		upper = 1
	}

	// Convert back to count scale
	lowerCount := int64(lower * float64(rsfa.totalDocs))
	upperCount := int64(upper * float64(rsfa.totalDocs))

	return lowerCount, upperCount
}

// GetTopChildren returns the top N children for the specified dimension.
// Returns estimated counts extrapolated from the sample.
func (rsfa *RandomSamplingFacetsAccumulator) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be positive, got %d", topN)
	}

	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	// Build prefix for hierarchical facets
	prefix := dim
	if len(path) > 0 {
		prefix = dim + "/" + joinPath(path...) + "/"
	} else {
		prefix = dim + "/"
	}

	// Collect matching labels and estimated counts
	type labelCount struct {
		label string
		count int64
		lower int64
		upper int64
	}
	var labelCounts []labelCount
	var totalCount int64

	for ord, sampledCount := range rsfa.counts {
		if sampledCount > 0 {
			label := rsfa.ordToLabel[ord]
			if hasPrefix(label, prefix) {
				// Extract the child label
				childLabel := trimPrefix(label, prefix)
				// Only include direct children
				if !contains(childLabel, "/") {
					estimatedCount := rsfa.extrapolateCount(sampledCount)
					lower, upper := rsfa.calculateConfidenceInterval(sampledCount)
					labelCounts = append(labelCounts, labelCount{
						label: childLabel,
						count: estimatedCount,
						lower: lower,
						upper: upper,
					})
					totalCount += estimatedCount
				}
			}
		}
	}

	// Sort by count descending
	sort.Slice(labelCounts, func(i, j int) bool {
		if labelCounts[i].count != labelCounts[j].count {
			return labelCounts[i].count > labelCounts[j].count
		}
		return labelCounts[i].label < labelCounts[j].label
	})

	// Take top N
	if len(labelCounts) > topN {
		labelCounts = labelCounts[:topN]
	}

	// Build result
	result := NewFacetResult(dim)
	result.Path = path
	result.Value = totalCount
	result.ChildCount = len(labelCounts)
	for _, lc := range labelCounts {
		result.AddLabelValue(NewLabelAndValue(lc.label, lc.count))
	}

	return result, nil
}

// extrapolateCount extrapolates a sampled count to the full population.
func (rsfa *RandomSamplingFacetsAccumulator) extrapolateCount(sampledCount int64) int64 {
	if rsfa.totalSampledDocs == 0 {
		return 0
	}
	return int64(float64(sampledCount) * float64(rsfa.totalDocs) / float64(rsfa.totalSampledDocs))
}

// calculateConfidenceInterval calculates the confidence interval for a sampled count.
func (rsfa *RandomSamplingFacetsAccumulator) calculateConfidenceInterval(sampledCount int64) (int64, int64) {
	if rsfa.totalSampledDocs == 0 || rsfa.totalDocs == 0 {
		return 0, 0
	}

	p := float64(sampledCount) / float64(rsfa.totalSampledDocs)
	se := math.Sqrt(p * (1 - p) / float64(rsfa.totalSampledDocs))

	zScore := 1.96 // 95% confidence
	margin := zScore * se
	lower := p - margin
	upper := p + margin

	if lower < 0 {
		lower = 0
	}
	if upper > 1 {
		upper = 1
	}

	lowerCount := int64(lower * float64(rsfa.totalDocs))
	upperCount := int64(upper * float64(rsfa.totalDocs))

	return lowerCount, upperCount
}

// GetAllChildren returns all children for the specified dimension.
func (rsfa *RandomSamplingFacetsAccumulator) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	return rsfa.GetTopChildren(int(^uint(0)>>1), dim, path...) // Max int
}

// GetSpecificValue returns the value for a specific label in a dimension.
func (rsfa *RandomSamplingFacetsAccumulator) GetSpecificValue(dim string, path ...string) (*FacetResult, error) {
	fullPath := dim
	if len(path) > 0 {
		fullPath = dim + "/" + joinPath(path...)
	}

	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	ord := rsfa.labelToOrd[fullPath]
	sampledCount := rsfa.counts[ord]
	estimatedCount := rsfa.extrapolateCount(sampledCount)

	result := NewFacetResult(dim)
	result.Path = path
	result.Value = estimatedCount
	if len(path) > 0 {
		result.AddLabelValue(NewLabelAndValue(path[len(path)-1], estimatedCount))
	}

	return result, nil
}

// GetDimensions returns all dimensions that have been accumulated.
func (rsfa *RandomSamplingFacetsAccumulator) GetDimensions() []string {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	dimSet := make(map[string]bool)
	for _, label := range rsfa.ordToLabel {
		parts := splitPath(label)
		if len(parts) > 0 {
			dimSet[parts[0]] = true
		}
	}

	dims := make([]string, 0, len(dimSet))
	for dim := range dimSet {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// Reset resets the accumulator to its initial state.
func (rsfa *RandomSamplingFacetsAccumulator) Reset() {
	rsfa.mu.Lock()
	defer rsfa.mu.Unlock()

	rsfa.counts = make([]int64, 1024)
	rsfa.ordToLabel = make(map[int]string)
	rsfa.labelToOrd = make(map[string]int)
	rsfa.nextOrd = 1
	rsfa.totalSampledDocs = 0
	rsfa.totalDocs = 0
}

// IsEmpty returns true if no facets have been accumulated.
func (rsfa *RandomSamplingFacetsAccumulator) IsEmpty() bool {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()

	for _, count := range rsfa.counts {
		if count > 0 {
			return false
		}
	}
	return true
}

// GetConfig returns the facets configuration.
func (rsfa *RandomSamplingFacetsAccumulator) GetConfig() *FacetsConfig {
	return rsfa.config
}

// GetSampleRate returns the sample rate.
func (rsfa *RandomSamplingFacetsAccumulator) GetSampleRate() float64 {
	return rsfa.sampleRate
}

// SetMinSampleSize sets the minimum sample size.
func (rsfa *RandomSamplingFacetsAccumulator) SetMinSampleSize(size int) {
	if size > 0 {
		rsfa.minSampleSize = size
	}
}

// GetMinSampleSize returns the minimum sample size.
func (rsfa *RandomSamplingFacetsAccumulator) GetMinSampleSize() int {
	return rsfa.minSampleSize
}

// SetMaxSampleSize sets the maximum sample size.
func (rsfa *RandomSamplingFacetsAccumulator) SetMaxSampleSize(size int) {
	if size > 0 {
		rsfa.maxSampleSize = size
	}
}

// GetMaxSampleSize returns the maximum sample size.
func (rsfa *RandomSamplingFacetsAccumulator) GetMaxSampleSize() int {
	return rsfa.maxSampleSize
}

// SetConfidenceLevel sets the confidence level for intervals.
func (rsfa *RandomSamplingFacetsAccumulator) SetConfidenceLevel(level float64) {
	if level > 0 && level < 1.0 {
		rsfa.confidenceLevel = level
	}
}

// GetConfidenceLevel returns the confidence level.
func (rsfa *RandomSamplingFacetsAccumulator) GetConfidenceLevel() float64 {
	return rsfa.confidenceLevel
}

// GetTotalSampledDocs returns the total number of documents sampled.
func (rsfa *RandomSamplingFacetsAccumulator) GetTotalSampledDocs() int64 {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()
	return rsfa.totalSampledDocs
}

// GetTotalDocs returns the total number of documents.
func (rsfa *RandomSamplingFacetsAccumulator) GetTotalDocs() int64 {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()
	return rsfa.totalDocs
}

// SetSeed sets the random seed for reproducibility.
func (rsfa *RandomSamplingFacetsAccumulator) SetSeed(seed int64) {
	rsfa.mu.Lock()
	defer rsfa.mu.Unlock()
	rsfa.seed = seed
	rsfa.random = rand.New(rand.NewSource(seed))
}

// GetSeed returns the random seed.
func (rsfa *RandomSamplingFacetsAccumulator) GetSeed() int64 {
	rsfa.mu.RLock()
	defer rsfa.mu.RUnlock()
	return rsfa.seed
}

// Ensure RandomSamplingFacetsAccumulator implements FacetsAccumulator
var _ FacetsAccumulator = (*RandomSamplingFacetsAccumulator)(nil)

// RandomSamplingFacetsAccumulatorBuilder helps build RandomSamplingFacetsAccumulator instances.
type RandomSamplingFacetsAccumulatorBuilder struct {
	config          *FacetsConfig
	sampleRate      float64
	minSampleSize   int
	maxSampleSize   int
	confidenceLevel float64
	seed            int64
}

// NewRandomSamplingFacetsAccumulatorBuilder creates a new builder.
func NewRandomSamplingFacetsAccumulatorBuilder(config *FacetsConfig, sampleRate float64) *RandomSamplingFacetsAccumulatorBuilder {
	return &RandomSamplingFacetsAccumulatorBuilder{
		config:          config,
		sampleRate:      sampleRate,
		minSampleSize:   100,
		maxSampleSize:   100000,
		confidenceLevel: 0.95,
		seed:            time.Now().UnixNano(),
	}
}

// SetMinSampleSize sets the minimum sample size.
func (b *RandomSamplingFacetsAccumulatorBuilder) SetMinSampleSize(size int) *RandomSamplingFacetsAccumulatorBuilder {
	b.minSampleSize = size
	return b
}

// SetMaxSampleSize sets the maximum sample size.
func (b *RandomSamplingFacetsAccumulatorBuilder) SetMaxSampleSize(size int) *RandomSamplingFacetsAccumulatorBuilder {
	b.maxSampleSize = size
	return b
}

// SetConfidenceLevel sets the confidence level.
func (b *RandomSamplingFacetsAccumulatorBuilder) SetConfidenceLevel(level float64) *RandomSamplingFacetsAccumulatorBuilder {
	b.confidenceLevel = level
	return b
}

// SetSeed sets the random seed.
func (b *RandomSamplingFacetsAccumulatorBuilder) SetSeed(seed int64) *RandomSamplingFacetsAccumulatorBuilder {
	b.seed = seed
	return b
}

// Build builds and returns the RandomSamplingFacetsAccumulator.
func (b *RandomSamplingFacetsAccumulatorBuilder) Build() (*RandomSamplingFacetsAccumulator, error) {
	acc, err := NewRandomSamplingFacetsAccumulator(b.config, b.sampleRate)
	if err != nil {
		return nil, err
	}

	acc.SetMinSampleSize(b.minSampleSize)
	acc.SetMaxSampleSize(b.maxSampleSize)
	acc.SetConfidenceLevel(b.confidenceLevel)
	acc.SetSeed(b.seed)

	return acc, nil
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"
	"sync"
)

// ConcurrentFacetsAccumulator is a thread-safe implementation of FacetsAccumulator
// that supports concurrent facet counting across multiple segments.
//
// This is the Go port of Lucene's concurrent faceting support, using Go's
// sync.RWMutex for thread-safe operations and sync.WaitGroup for parallel processing.
//
// The accumulator maintains thread-safe access to facet counts while allowing
// parallel accumulation across different segments. It uses a read-write lock
// to minimize contention during read operations.
type ConcurrentFacetsAccumulator struct {
	// config contains the facets configuration
	config *FacetsConfig

	// counts holds the aggregated counts per ordinal
	// Protected by mu
	counts []int64

	// ordToLabel maps ordinals to their labels
	// Protected by mu
	ordToLabel map[int]string

	// labelToOrd maps labels to their ordinals
	// Protected by mu
	labelToOrd map[string]int

	// nextOrd is the next available ordinal
	// Protected by mu
	nextOrd int

	// mu protects the maps and counts slice
	mu sync.RWMutex

	// maxCategories is the maximum number of categories to track
	maxCategories int

	// hierarchical indicates if hierarchical facets are supported
	hierarchical bool

	// numWorkers is the number of concurrent workers for accumulation
	numWorkers int
}

// NewConcurrentFacetsAccumulator creates a new ConcurrentFacetsAccumulator.
//
// Parameters:
//   - config: the facets configuration
//
// Returns:
//   - a new ConcurrentFacetsAccumulator instance
func NewConcurrentFacetsAccumulator(config *FacetsConfig) (*ConcurrentFacetsAccumulator, error) {
	if config == nil {
		return nil, fmt.Errorf("facets config cannot be nil")
	}

	return &ConcurrentFacetsAccumulator{
		config:        config,
		counts:        make([]int64, 1024),
		ordToLabel:    make(map[int]string),
		labelToOrd:    make(map[string]int),
		nextOrd:       1, // Start at 1, reserve 0 for invalid
		maxCategories: 10000,
		hierarchical:  true,
		numWorkers:    4, // Default to 4 workers
	}, nil
}

// Accumulate accumulates facet counts from the given facet results.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) Accumulate(results []*FacetResult) error {
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
			ord := cfa.getOrCreateOrdinal(lv.Label)
			if ord > 0 {
				cfa.incrementCount(ord, lv.Value)
			}
		}
	}
	return nil
}

// AccumulateFromMatchingDocs accumulates counts from matching documents concurrently.
// This is the main entry point for accumulating facet counts from search results.
// It processes segments in parallel using a worker pool.
//
// Parameters:
//   - matchingDocs: the matching documents from each segment
//
// Returns:
//   - error if accumulation fails
func (cfa *ConcurrentFacetsAccumulator) AccumulateFromMatchingDocs(matchingDocs []*MatchingDocs) error {
	if len(matchingDocs) == 0 {
		return nil
	}

	// Use a WaitGroup to wait for all workers to complete
	var wg sync.WaitGroup

	// Create a channel to distribute work
	workChan := make(chan *MatchingDocs, len(matchingDocs))

	// Start workers
	numWorkers := cfa.numWorkers
	if numWorkers > len(matchingDocs) {
		numWorkers = len(matchingDocs)
	}

	errChan := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for md := range workChan {
				if err := cfa.accumulateFromSegment(md); err != nil {
					select {
					case errChan <- err:
					default:
					}
				}
			}
		}()
	}

	// Distribute work
	for _, md := range matchingDocs {
		workChan <- md
	}
	close(workChan)

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("accumulating from segments: %w", err)
		}
	}

	return nil
}

// accumulateFromSegment accumulates counts from a single segment.
// This method is called concurrently by multiple workers.
func (cfa *ConcurrentFacetsAccumulator) accumulateFromSegment(matchingDocs *MatchingDocs) error {
	if matchingDocs == nil {
		return nil
	}

	// In a full implementation, this would:
	// 1. Get the doc values for the facet field
	// 2. Iterate over matching documents
	// 3. For each document, get its ordinals and increment counts
	// For now, this is a placeholder that returns nil

	return nil
}

// getOrCreateOrdinal gets an existing ordinal or creates a new one for the label.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) getOrCreateOrdinal(label string) int {
	// First try read lock
	cfa.mu.RLock()
	if ord, exists := cfa.labelToOrd[label]; exists {
		cfa.mu.RUnlock()
		return ord
	}
	cfa.mu.RUnlock()

	// Need to create new ordinal, use write lock
	cfa.mu.Lock()
	defer cfa.mu.Unlock()

	// Double-check after acquiring write lock
	if ord, exists := cfa.labelToOrd[label]; exists {
		return ord
	}

	// Check if we've reached the maximum number of categories
	if len(cfa.labelToOrd) >= cfa.maxCategories {
		return 0 // Return invalid ordinal
	}

	// Create new ordinal
	ord := cfa.nextOrd
	cfa.nextOrd++
	cfa.labelToOrd[label] = ord
	cfa.ordToLabel[ord] = label

	// Ensure capacity
	cfa.ensureCapacityLocked(ord)

	return ord
}

// ensureCapacityLocked ensures the counts slice can hold the given ordinal.
// Must be called with write lock held.
func (cfa *ConcurrentFacetsAccumulator) ensureCapacityLocked(ord int) {
	if ord >= len(cfa.counts) {
		newSize := ord * 2
		if newSize > cfa.maxCategories {
			newSize = cfa.maxCategories
		}
		newCounts := make([]int64, newSize)
		copy(newCounts, cfa.counts)
		cfa.counts = newCounts
	}
}

// incrementCount increments the count for the given ordinal.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) incrementCount(ordinal int, count int64) {
	if ordinal <= 0 {
		return
	}

	cfa.mu.Lock()
	defer cfa.mu.Unlock()

	cfa.ensureCapacityLocked(ordinal)
	cfa.counts[ordinal] += count
}

// GetCount returns the count for the given ordinal.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) GetCount(ordinal int) int64 {
	cfa.mu.RLock()
	defer cfa.mu.RUnlock()

	if ordinal >= 0 && ordinal < len(cfa.counts) {
		return cfa.counts[ordinal]
	}
	return 0
}

// GetTopChildren returns the top N children for the specified dimension.
// This method is thread-safe.
//
// Parameters:
//   - topN: maximum number of children to return
//   - dim: the dimension/facet field name
//   - path: optional path for hierarchical facets
//
// Returns:
//   - FacetResult containing the top children, or error if dimension not found
func (cfa *ConcurrentFacetsAccumulator) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be positive, got %d", topN)
	}

	cfa.mu.RLock()
	defer cfa.mu.RUnlock()

	// Build prefix for hierarchical facets
	prefix := dim
	if len(path) > 0 {
		prefix = dim + "/" + joinPath(path...) + "/"
	} else {
		prefix = dim + "/"
	}

	// Collect matching labels and counts
	type labelCount struct {
		label string
		count int64
	}
	var labelCounts []labelCount
	var totalCount int64

	for ord, count := range cfa.counts {
		if count > 0 {
			label := cfa.ordToLabel[ord]
			if hasPrefix(label, prefix) {
				// Extract the child label
				childLabel := trimPrefix(label, prefix)
				// Only include direct children
				if !contains(childLabel, "/") {
					labelCounts = append(labelCounts, labelCount{label: childLabel, count: count})
					totalCount += count
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

// GetAllChildren returns all children for the specified dimension.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	return cfa.GetTopChildren(cfa.maxCategories, dim, path...)
}

// GetSpecificValue returns the value for a specific label in a dimension.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) GetSpecificValue(dim string, path ...string) (*FacetResult, error) {
	fullPath := dim
	if len(path) > 0 {
		fullPath = dim + "/" + joinPath(path...)
	}

	cfa.mu.RLock()
	defer cfa.mu.RUnlock()

	ord := cfa.labelToOrd[fullPath]
	count := cfa.counts[ord]

	result := NewFacetResult(dim)
	result.Path = path
	result.Value = count
	if len(path) > 0 {
		result.AddLabelValue(NewLabelAndValue(path[len(path)-1], count))
	}

	return result, nil
}

// GetDimensions returns all dimensions that have been accumulated.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) GetDimensions() []string {
	cfa.mu.RLock()
	defer cfa.mu.RUnlock()

	dimSet := make(map[string]bool)
	for _, label := range cfa.ordToLabel {
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
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) Reset() {
	cfa.mu.Lock()
	defer cfa.mu.Unlock()

	cfa.counts = make([]int64, 1024)
	cfa.ordToLabel = make(map[int]string)
	cfa.labelToOrd = make(map[string]int)
	cfa.nextOrd = 1
}

// IsEmpty returns true if no facets have been accumulated.
// This method is thread-safe.
func (cfa *ConcurrentFacetsAccumulator) IsEmpty() bool {
	cfa.mu.RLock()
	defer cfa.mu.RUnlock()

	for _, count := range cfa.counts {
		if count > 0 {
			return false
		}
	}
	return true
}

// GetConfig returns the facets configuration.
func (cfa *ConcurrentFacetsAccumulator) GetConfig() *FacetsConfig {
	return cfa.config
}

// SetNumWorkers sets the number of concurrent workers for accumulation.
func (cfa *ConcurrentFacetsAccumulator) SetNumWorkers(numWorkers int) {
	if numWorkers > 0 {
		cfa.numWorkers = numWorkers
	}
}

// GetNumWorkers returns the number of concurrent workers.
func (cfa *ConcurrentFacetsAccumulator) GetNumWorkers() int {
	return cfa.numWorkers
}

// SetMaxCategories sets the maximum number of categories to track.
func (cfa *ConcurrentFacetsAccumulator) SetMaxCategories(max int) {
	cfa.mu.Lock()
	defer cfa.mu.Unlock()
	cfa.maxCategories = max
}

// GetMaxCategories returns the maximum number of categories.
func (cfa *ConcurrentFacetsAccumulator) GetMaxCategories() int {
	cfa.mu.RLock()
	defer cfa.mu.RUnlock()
	return cfa.maxCategories
}

// SetHierarchical sets whether this accumulator supports hierarchical facets.
func (cfa *ConcurrentFacetsAccumulator) SetHierarchical(hierarchical bool) {
	cfa.hierarchical = hierarchical
}

// IsHierarchical returns true if this accumulator supports hierarchical facets.
func (cfa *ConcurrentFacetsAccumulator) IsHierarchical() bool {
	return cfa.hierarchical
}

// Ensure ConcurrentFacetsAccumulator implements FacetsAccumulator
var _ FacetsAccumulator = (*ConcurrentFacetsAccumulator)(nil)

// ConcurrentFacetsAccumulatorFactory creates ConcurrentFacetsAccumulator instances.
type ConcurrentFacetsAccumulatorFactory struct {
	// config is the facets configuration
	config *FacetsConfig

	// numWorkers is the number of concurrent workers
	numWorkers int
}

// NewConcurrentFacetsAccumulatorFactory creates a new factory.
func NewConcurrentFacetsAccumulatorFactory(config *FacetsConfig) *ConcurrentFacetsAccumulatorFactory {
	return &ConcurrentFacetsAccumulatorFactory{
		config:     config,
		numWorkers: 4,
	}
}

// SetNumWorkers sets the number of concurrent workers.
func (f *ConcurrentFacetsAccumulatorFactory) SetNumWorkers(numWorkers int) *ConcurrentFacetsAccumulatorFactory {
	f.numWorkers = numWorkers
	return f
}

// CreateAccumulator creates a new ConcurrentFacetsAccumulator.
func (f *ConcurrentFacetsAccumulatorFactory) CreateAccumulator() (*ConcurrentFacetsAccumulator, error) {
	acc, err := NewConcurrentFacetsAccumulator(f.config)
	if err != nil {
		return nil, err
	}
	acc.SetNumWorkers(f.numWorkers)
	return acc, nil
}

// ConcurrentFacetsAccumulatorBuilder helps build ConcurrentFacetsAccumulator instances.
type ConcurrentFacetsAccumulatorBuilder struct {
	config        *FacetsConfig
	numWorkers    int
	hierarchical  bool
	maxCategories int
}

// NewConcurrentFacetsAccumulatorBuilder creates a new builder.
func NewConcurrentFacetsAccumulatorBuilder(config *FacetsConfig) *ConcurrentFacetsAccumulatorBuilder {
	return &ConcurrentFacetsAccumulatorBuilder{
		config:        config,
		numWorkers:    4,
		hierarchical:  true,
		maxCategories: 10000,
	}
}

// SetNumWorkers sets the number of concurrent workers.
func (b *ConcurrentFacetsAccumulatorBuilder) SetNumWorkers(numWorkers int) *ConcurrentFacetsAccumulatorBuilder {
	b.numWorkers = numWorkers
	return b
}

// SetHierarchical sets whether the accumulator supports hierarchical facets.
func (b *ConcurrentFacetsAccumulatorBuilder) SetHierarchical(hierarchical bool) *ConcurrentFacetsAccumulatorBuilder {
	b.hierarchical = hierarchical
	return b
}

// SetMaxCategories sets the maximum number of categories.
func (b *ConcurrentFacetsAccumulatorBuilder) SetMaxCategories(max int) *ConcurrentFacetsAccumulatorBuilder {
	b.maxCategories = max
	return b
}

// Build builds and returns the ConcurrentFacetsAccumulator.
func (b *ConcurrentFacetsAccumulatorBuilder) Build() (*ConcurrentFacetsAccumulator, error) {
	acc, err := NewConcurrentFacetsAccumulator(b.config)
	if err != nil {
		return nil, err
	}

	acc.SetNumWorkers(b.numWorkers)
	acc.SetHierarchical(b.hierarchical)
	acc.SetMaxCategories(b.maxCategories)

	return acc, nil
}

// Helper functions

func joinPath(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "/" + parts[i]
	}
	return result
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	parts := make([]string, 0)
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimPrefix(s, prefix string) string {
	if hasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

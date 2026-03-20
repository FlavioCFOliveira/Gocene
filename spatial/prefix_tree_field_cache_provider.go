// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FieldCacheProvider provides cached access to field values for spatial operations.
// This is used to optimize repeated access to document values during spatial queries
// and value source calculations.
//
// In Lucene, the FieldCache is a mechanism for caching field values in memory
// to avoid repeated disk access. For spatial operations, we cache the parsed
// spatial cells/tokens for quick lookup.
//
// This is the Go port of Lucene's FieldCache concept adapted for spatial prefix trees.
type FieldCacheProvider struct {
	// cache stores field values per segment reader
	cache map[string]*FieldCacheEntry
	mu    sync.RWMutex
}

// FieldCacheEntry represents cached values for a specific field in a segment.
type FieldCacheEntry struct {
	// fieldName is the name of the cached field
	fieldName string

	// readerKey identifies the index reader
	readerKey string

	// cellTokens stores the cell tokens for each document
	// The outer slice is indexed by docID, inner slice contains tokens for that doc
	cellTokens [][]string

	// docCount is the number of documents in this segment
	docCount int

	// prefixTree is the spatial prefix tree used for parsing tokens
	prefixTree SpatialPrefixTree

	// hasValues tracks which documents have values
	hasValues []bool
}

// SpatialPrefixTreeFieldCacheProvider extends FieldCacheProvider with spatial-specific functionality.
// It provides efficient caching of spatial cell data for prefix tree strategies.
//
// This is the Go port of Lucene's spatial FieldCacheProvider.
type SpatialPrefixTreeFieldCacheProvider struct {
	*FieldCacheProvider
	prefixTree SpatialPrefixTree
	fieldName  string
}

// NewFieldCacheProvider creates a new FieldCacheProvider.
func NewFieldCacheProvider() *FieldCacheProvider {
	return &FieldCacheProvider{
		cache: make(map[string]*FieldCacheEntry),
	}
}

// NewSpatialPrefixTreeFieldCacheProvider creates a new SpatialPrefixTreeFieldCacheProvider.
//
// Parameters:
//   - fieldName: The name of the field containing spatial tokens
//   - prefixTree: The spatial prefix tree for parsing tokens
//
// Returns a configured cache provider for spatial prefix tree fields.
func NewSpatialPrefixTreeFieldCacheProvider(fieldName string, prefixTree SpatialPrefixTree) (*SpatialPrefixTreeFieldCacheProvider, error) {
	if fieldName == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if prefixTree == nil {
		return nil, fmt.Errorf("prefix tree cannot be nil")
	}

	return &SpatialPrefixTreeFieldCacheProvider{
		FieldCacheProvider: NewFieldCacheProvider(),
		prefixTree:         prefixTree,
		fieldName:          fieldName,
	}, nil
}

// GetFieldName returns the field name for this cache provider.
func (p *SpatialPrefixTreeFieldCacheProvider) GetFieldName() string {
	return p.fieldName
}

// GetPrefixTree returns the spatial prefix tree.
func (p *SpatialPrefixTreeFieldCacheProvider) GetPrefixTree() SpatialPrefixTree {
	return p.prefixTree
}

// GetCacheEntry retrieves or creates a cache entry for the given reader.
//
// Parameters:
//   - reader: The index reader to cache values for
//
// Returns a FieldCacheEntry containing cached values for the reader.
func (p *SpatialPrefixTreeFieldCacheProvider) GetCacheEntry(reader *index.IndexReader) (*FieldCacheEntry, error) {
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}

	// Generate a unique key for this reader
	readerKey := generateReaderKey(reader)
	cacheKey := p.fieldName + "_" + readerKey

	// Check if we have a cached entry
	p.mu.RLock()
	entry, exists := p.cache[cacheKey]
	p.mu.RUnlock()

	if exists {
		return entry, nil
	}

	// Create new entry
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists := p.cache[cacheKey]; exists {
		return entry, nil
	}

	entry, err := p.createCacheEntry(reader)
	if err != nil {
		return nil, err
	}

	p.cache[cacheKey] = entry
	return entry, nil
}

// createCacheEntry creates a new cache entry by reading values from the index.
func (p *SpatialPrefixTreeFieldCacheProvider) createCacheEntry(reader *index.IndexReader) (*FieldCacheEntry, error) {
	maxDoc := reader.MaxDoc()
	entry := &FieldCacheEntry{
		fieldName:  p.fieldName,
		readerKey:  generateReaderKey(reader),
		cellTokens: make([][]string, maxDoc),
		docCount:   maxDoc,
		prefixTree: p.prefixTree,
		hasValues:  make([]bool, maxDoc),
	}

	// Get the term iterator for the field
	terms, err := reader.Terms(p.fieldName)
	if err != nil {
		return nil, fmt.Errorf("failed to get terms for field %s: %w", p.fieldName, err)
	}

	if terms == nil {
		// Field doesn't exist in this segment
		return entry, nil
	}

	// Iterate through all terms and their documents
	// In a full implementation, this would use the PostingsEnum to get doc IDs
	// For now, we create a placeholder that can be populated on demand

	return entry, nil
}

// GetCellTokens returns the cell tokens for a specific document.
//
// Parameters:
//   - docID: The document ID to get tokens for
//   - reader: The index reader
//
// Returns the slice of cell tokens for the document.
func (p *SpatialPrefixTreeFieldCacheProvider) GetCellTokens(docID int, reader *index.IndexReader) ([]string, error) {
	if docID < 0 {
		return nil, fmt.Errorf("docID cannot be negative")
	}
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}

	entry, err := p.GetCacheEntry(reader)
	if err != nil {
		return nil, err
	}

	if docID >= entry.docCount {
		return nil, fmt.Errorf("docID %d exceeds max document %d", docID, entry.docCount)
	}

	// Return cached tokens if available
	if entry.cellTokens[docID] != nil {
		return entry.cellTokens[docID], nil
	}

	// Otherwise, load from index (lazy loading)
	tokens, err := p.loadTokensForDoc(docID, reader)
	if err != nil {
		return nil, err
	}

	entry.cellTokens[docID] = tokens
	entry.hasValues[docID] = len(tokens) > 0

	return tokens, nil
}

// loadTokensForDoc loads cell tokens from the index for a specific document.
func (p *SpatialPrefixTreeFieldCacheProvider) loadTokensForDoc(docID int, reader *index.IndexReader) ([]string, error) {
	// In a full implementation, this would:
	// 1. Get the term vector or doc values for the field
	// 2. Parse the stored tokens
	// 3. Return as a slice of strings

	// For now, return an empty slice as placeholder
	// The actual implementation would depend on how the spatial data is stored
	return []string{}, nil
}

// HasValues returns true if the document has cached values.
func (p *SpatialPrefixTreeFieldCacheProvider) HasValues(docID int, reader *index.IndexReader) bool {
	entry, err := p.GetCacheEntry(reader)
	if err != nil {
		return false
	}

	if docID < 0 || docID >= entry.docCount {
		return false
	}

	return entry.hasValues[docID]
}

// GetCell returns the parsed Cell for a document's token at the given index.
//
// Parameters:
//   - docID: The document ID
//   - tokenIndex: The index of the token in the document's token list
//   - reader: The index reader
//
// Returns the parsed Cell or an error if parsing fails.
func (p *SpatialPrefixTreeFieldCacheProvider) GetCell(docID int, tokenIndex int, reader *index.IndexReader) (Cell, error) {
	tokens, err := p.GetCellTokens(docID, reader)
	if err != nil {
		return nil, err
	}

	if tokenIndex < 0 || tokenIndex >= len(tokens) {
		return nil, fmt.Errorf("token index %d out of range (have %d tokens)", tokenIndex, len(tokens))
	}

	return p.prefixTree.GetCell(tokens[tokenIndex])
}

// GetAllCells returns all parsed Cells for a document.
//
// Parameters:
//   - docID: The document ID
//   - reader: The index reader
//
// Returns a slice of parsed Cells.
func (p *SpatialPrefixTreeFieldCacheProvider) GetAllCells(docID int, reader *index.IndexReader) ([]Cell, error) {
	tokens, err := p.GetCellTokens(docID, reader)
	if err != nil {
		return nil, err
	}

	cells := make([]Cell, 0, len(tokens))
	for _, token := range tokens {
		cell, err := p.prefixTree.GetCell(token)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cell token %s: %w", token, err)
		}
		cells = append(cells, cell)
	}

	return cells, nil
}

// Invalidate clears the cache for a specific reader.
func (p *SpatialPrefixTreeFieldCacheProvider) Invalidate(reader *index.IndexReader) {
	if reader == nil {
		return
	}

	readerKey := generateReaderKey(reader)
	cacheKey := p.fieldName + "_" + readerKey

	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.cache, cacheKey)
}

// InvalidateAll clears all cached entries.
func (p *SpatialPrefixTreeFieldCacheProvider) InvalidateAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cache = make(map[string]*FieldCacheEntry)
}

// CacheSize returns the number of cached entries.
func (p *SpatialPrefixTreeFieldCacheProvider) CacheSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.cache)
}

// generateReaderKey generates a unique key for an index reader.
// In a real implementation, this would use the reader's identity or version.
func generateReaderKey(reader *index.IndexReader) string {
	if reader == nil {
		return "r0_0"
	}
	// Use the reader's pointer address and max doc as a simple key
	// In production, this should use a proper reader identifier
	return fmt.Sprintf("r%d_%d", reader.MaxDoc(), reader.NumDocs())
}

// SpatialFieldCacheValueSource provides a ValueSource that uses the field cache.
// This enables efficient sorting and faceting by spatial values.
type SpatialFieldCacheValueSource struct {
	cacheProvider *SpatialPrefixTreeFieldCacheProvider
	center        Point
	multiplier    float64
}

// NewSpatialFieldCacheValueSource creates a new value source using the field cache.
func NewSpatialFieldCacheValueSource(cacheProvider *SpatialPrefixTreeFieldCacheProvider, center Point, multiplier float64) (*SpatialFieldCacheValueSource, error) {
	if cacheProvider == nil {
		return nil, fmt.Errorf("cache provider cannot be nil")
	}

	return &SpatialFieldCacheValueSource{
		cacheProvider: cacheProvider,
		center:        center,
		multiplier:    multiplier,
	}, nil
}

// Description returns a description of this value source.
func (s *SpatialFieldCacheValueSource) Description() string {
	return fmt.Sprintf("spatial_field_cache(%s from %v)", s.cacheProvider.fieldName, s.center)
}

// PrefixTreeAwareQuery is a marker interface for queries that can use the prefix tree cache.
type PrefixTreeAwareQuery interface {
	search.Query
	// SetFieldCacheProvider sets the field cache provider for this query.
	SetFieldCacheProvider(provider *SpatialPrefixTreeFieldCacheProvider)
	// GetFieldCacheProvider returns the field cache provider.
	GetFieldCacheProvider() *SpatialPrefixTreeFieldCacheProvider
}

// Ensure SpatialPrefixTreeFieldCacheProvider is properly implemented
var _ PrefixTreeAwareQuery = (*IntersectsPrefixTreeQuery)(nil)

// SetFieldCacheProvider sets the field cache provider for IntersectsPrefixTreeQuery.
func (q *IntersectsPrefixTreeQuery) SetFieldCacheProvider(provider *SpatialPrefixTreeFieldCacheProvider) {
	q.fieldCacheProvider = provider
}

// GetFieldCacheProvider returns the field cache provider.
func (q *IntersectsPrefixTreeQuery) GetFieldCacheProvider() *SpatialPrefixTreeFieldCacheProvider {
	return q.fieldCacheProvider
}

// CacheStats provides statistics about the field cache.
type CacheStats struct {
	// EntryCount is the number of cache entries.
	EntryCount int

	// TotalDocs is the total number of documents cached.
	TotalDocs int

	// TotalTokens is the total number of cell tokens cached.
	TotalTokens int

	// MemoryEstimate is an estimate of memory usage in bytes.
	MemoryEstimate int64
}

// GetStats returns statistics about the cache.
func (p *SpatialPrefixTreeFieldCacheProvider) GetStats() *CacheStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := &CacheStats{
		EntryCount: len(p.cache),
	}

	for _, entry := range p.cache {
		stats.TotalDocs += entry.docCount
		for _, tokens := range entry.cellTokens {
			stats.TotalTokens += len(tokens)
			// Estimate memory: 8 bytes per slice header + 16 bytes per string
			stats.MemoryEstimate += int64(len(tokens) * 24)
		}
		// Base entry overhead
		stats.MemoryEstimate += int64(entry.docCount * 2) // hasValues bool per doc
	}

	return stats
}

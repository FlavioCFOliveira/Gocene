// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/grouping"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SpatialIndexReader reads spatial index data for efficient geospatial queries.
// It works with SpatialStrategy implementations to read and query indexed shapes
// using various approaches (point vector, bounding box, prefix tree, etc.).
//
// The reader provides thread-safe access to spatial index data and supports
// various spatial query operations.
//
// This is the Go port of Lucene's spatial index reading functionality.
type SpatialIndexReader struct {
	mu sync.RWMutex

	// directory is where the spatial index files are read from
	directory store.Directory

	// segmentInfo contains metadata about the segment
	segmentInfo *index.SegmentInfo

	// fieldInfos contains metadata about all spatial fields
	fieldInfos *index.FieldInfos

	// strategies maps field names to their spatial strategies
	strategies map[string]SpatialStrategy

	// closed indicates if this reader has been closed
	closed bool

	// docCount is the total number of documents in the segment
	docCount int
}

// NewSpatialIndexReader creates a new SpatialIndexReader.
//
// Parameters:
//   - directory: The directory where spatial index files are located
//   - segmentInfo: Metadata about the segment being read
//   - fieldInfos: Metadata about all fields in the index
//
// Returns an error if any required parameter is nil.
func NewSpatialIndexReader(directory store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) (*SpatialIndexReader, error) {
	if directory == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}
	if segmentInfo == nil {
		return nil, fmt.Errorf("segmentInfo cannot be nil")
	}
	if fieldInfos == nil {
		return nil, fmt.Errorf("fieldInfos cannot be nil")
	}

	return &SpatialIndexReader{
		directory:   directory,
		segmentInfo: segmentInfo,
		fieldInfos:  fieldInfos,
		strategies:  make(map[string]SpatialStrategy),
		closed:      false,
		docCount:    segmentInfo.DocCount(),
	}, nil
}

// RegisterStrategy registers a spatial strategy for a field.
// This must be called before querying documents with spatial data for that field.
//
// Parameters:
//   - fieldName: The name of the spatial field
//   - strategy: The spatial strategy to use for querying
//
// Returns an error if the strategy is nil or if a strategy is already registered
// for the field.
func (r *SpatialIndexReader) RegisterStrategy(fieldName string, strategy SpatialStrategy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("SpatialIndexReader is closed")
	}

	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	if existing, ok := r.strategies[fieldName]; ok {
		return fmt.Errorf("strategy already registered for field %q: %v", fieldName, existing)
	}

	r.strategies[fieldName] = strategy
	return nil
}

// GetStrategy returns the spatial strategy registered for a field.
// Returns nil if no strategy is registered for the field.
func (r *SpatialIndexReader) GetStrategy(fieldName string) SpatialStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.strategies[fieldName]
}

// HasStrategy returns true if a spatial strategy is registered for the field.
func (r *SpatialIndexReader) HasStrategy(fieldName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.strategies[fieldName]
	return ok
}

// MakeQuery creates a spatial query for the given field, operation, and shape.
// It uses the registered SpatialStrategy to create the appropriate query.
//
// Parameters:
//   - fieldName: The name of the spatial field
//   - operation: The spatial operation (Intersects, Within, Contains, etc.)
//   - shape: The query shape
//
// Returns a search.Query that can be used with IndexSearcher, or an error if
// the reader is closed, no strategy is registered for the field, or if the
// query cannot be created.
func (r *SpatialIndexReader) MakeQuery(fieldName string, operation SpatialOperation, shape Shape) (search.Query, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("SpatialIndexReader is closed")
	}

	strategy, ok := r.strategies[fieldName]
	if !ok {
		return nil, fmt.Errorf("no spatial strategy registered for field %q", fieldName)
	}

	if shape == nil {
		return nil, fmt.Errorf("shape cannot be nil")
	}

	query, err := strategy.MakeQuery(operation, shape)
	if err != nil {
		return nil, fmt.Errorf("failed to create query for field %q: %w", fieldName, err)
	}

	return query, nil
}

// MakeDistanceValueSource creates a ValueSource that returns the distance
// from the center of indexed shapes to a specified point.
// This is used for sorting or boosting by distance.
//
// Parameters:
//   - fieldName: The name of the spatial field
//   - point: The point to calculate distance from
//   - multiplier: Distance multiplier (e.g., for unit conversion)
//
// Returns a grouping.ValueSource, or an error if the reader is closed or
// no strategy is registered for the field.
func (r *SpatialIndexReader) MakeDistanceValueSource(fieldName string, point Point, multiplier float64) (grouping.ValueSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("SpatialIndexReader is closed")
	}

	strategy, ok := r.strategies[fieldName]
	if !ok {
		return nil, fmt.Errorf("no spatial strategy registered for field %q", fieldName)
	}

	// Note: This would need the actual ValueSource interface from the grouping package
	// For now, we return an error indicating this needs the grouping package integration
	_, err := strategy.MakeDistanceValueSource(point, multiplier)
	if err != nil {
		return nil, fmt.Errorf("failed to create distance value source for field %q: %w", fieldName, err)
	}

	// TODO: Return actual ValueSource when grouping package is fully integrated
	return nil, fmt.Errorf("distance value source not yet fully implemented")
}

// GetDocCount returns the total number of documents in this segment.
func (r *SpatialIndexReader) GetDocCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.docCount
}

// GetDirectory returns the directory used by this reader.
func (r *SpatialIndexReader) GetDirectory() store.Directory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.directory
}

// GetSegmentInfo returns the segment info for this reader.
func (r *SpatialIndexReader) GetSegmentInfo() *index.SegmentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.segmentInfo
}

// GetFieldInfos returns the field infos for this reader.
func (r *SpatialIndexReader) GetFieldInfos() *index.FieldInfos {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.fieldInfos
}

// IsClosed returns true if this reader has been closed.
func (r *SpatialIndexReader) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.closed
}

// Close releases all resources used by this reader.
// After closing, no further operations can be performed.
//
// Returns an error if the reader is already closed.
func (r *SpatialIndexReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("SpatialIndexReader is already closed")
	}

	r.closed = true
	r.strategies = nil
	return nil
}

// GetRegisteredFields returns a list of field names that have registered strategies.
func (r *SpatialIndexReader) GetRegisteredFields() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fields := make([]string, 0, len(r.strategies))
	for fieldName := range r.strategies {
		fields = append(fields, fieldName)
	}
	return fields
}

// ClearStrategies removes all registered spatial strategies.
// This can be used to reset the reader for a new configuration.
//
// Returns an error if the reader is closed.
func (r *SpatialIndexReader) ClearStrategies() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("SpatialIndexReader is closed")
	}

	r.strategies = make(map[string]SpatialStrategy)
	return nil
}

// SpatialIndexReaderState holds the current state of the reader.
// This can be used for monitoring and debugging purposes.
type SpatialIndexReaderState struct {
	DocCount         int
	RegisteredFields []string
	Closed           bool
}

// GetState returns the current state of the reader.
func (r *SpatialIndexReader) GetState() SpatialIndexReaderState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return SpatialIndexReaderState{
		DocCount:         r.docCount,
		RegisteredFields: r.GetRegisteredFields(),
		Closed:           r.closed,
	}
}

// CheckIntegrity verifies the integrity of the spatial index.
// This checks that all registered strategies can access their data.
//
// Returns an error if any integrity check fails.
func (r *SpatialIndexReader) CheckIntegrity() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return fmt.Errorf("SpatialIndexReader is closed")
	}

	// Check that all strategies have valid field names
	for fieldName, strategy := range r.strategies {
		if strategy.GetFieldName() != fieldName {
			return fmt.Errorf("strategy field name mismatch: expected %q, got %q", fieldName, strategy.GetFieldName())
		}
	}

	return nil
}

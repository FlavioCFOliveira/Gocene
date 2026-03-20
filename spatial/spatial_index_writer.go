// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SpatialIndexWriter writes spatial index data for efficient geospatial queries.
// It works with SpatialStrategy implementations to index shapes using various
// approaches (point vector, bounding box, prefix tree, etc.).
//
// The writer maintains state for the current segment being written and provides
// thread-safe operations for concurrent document indexing.
//
// This is the Go port of Lucene's spatial index writing functionality.
type SpatialIndexWriter struct {
	mu sync.RWMutex

	// directory is where the spatial index files are written
	directory store.Directory

	// segmentInfo contains metadata about the current segment
	segmentInfo *index.SegmentInfo

	// fieldInfos contains metadata about all spatial fields
	fieldInfos *index.FieldInfos

	// strategies maps field names to their spatial strategies
	strategies map[string]SpatialStrategy

	// closed indicates if this writer has been closed
	closed bool

	// currentDoc is the current document being written
	currentDoc int

	// docCount is the total number of documents written
	docCount int
}

// NewSpatialIndexWriter creates a new SpatialIndexWriter.
//
// Parameters:
//   - directory: The directory where spatial index files will be written
//   - segmentInfo: Metadata about the segment being written
//   - fieldInfos: Metadata about all fields in the index
//
// Returns an error if any required parameter is nil.
func NewSpatialIndexWriter(directory store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) (*SpatialIndexWriter, error) {
	if directory == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}
	if segmentInfo == nil {
		return nil, fmt.Errorf("segmentInfo cannot be nil")
	}
	if fieldInfos == nil {
		return nil, fmt.Errorf("fieldInfos cannot be nil")
	}

	return &SpatialIndexWriter{
		directory:   directory,
		segmentInfo: segmentInfo,
		fieldInfos:  fieldInfos,
		strategies:  make(map[string]SpatialStrategy),
		currentDoc:  -1,
		docCount:    0,
		closed:      false,
	}, nil
}

// RegisterStrategy registers a spatial strategy for a field.
// This must be called before writing documents with spatial data for that field.
//
// Parameters:
//   - fieldName: The name of the spatial field
//   - strategy: The spatial strategy to use for indexing
//
// Returns an error if the strategy is nil or if a strategy is already registered
// for the field.
func (w *SpatialIndexWriter) RegisterStrategy(fieldName string, strategy SpatialStrategy) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("SpatialIndexWriter is closed")
	}

	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	if existing, ok := w.strategies[fieldName]; ok {
		return fmt.Errorf("strategy already registered for field %q: %v", fieldName, existing)
	}

	w.strategies[fieldName] = strategy
	return nil
}

// GetStrategy returns the spatial strategy registered for a field.
// Returns nil if no strategy is registered for the field.
func (w *SpatialIndexWriter) GetStrategy(fieldName string) SpatialStrategy {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.strategies[fieldName]
}

// HasStrategy returns true if a spatial strategy is registered for the field.
func (w *SpatialIndexWriter) HasStrategy(fieldName string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	_, ok := w.strategies[fieldName]
	return ok
}

// StartDocument starts writing a new document.
// This must be called before writing any spatial fields for the document.
//
// Returns an error if the writer is closed.
func (w *SpatialIndexWriter) StartDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("SpatialIndexWriter is closed")
	}

	w.currentDoc++
	return nil
}

// FinishDocument finishes writing the current document.
// This should be called after all spatial fields for the document have been written.
//
// Returns an error if the writer is closed.
func (w *SpatialIndexWriter) FinishDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("SpatialIndexWriter is closed")
	}

	w.docCount++
	return nil
}

// WriteSpatialField writes a spatial field to the index.
// It uses the registered SpatialStrategy to create the appropriate indexable fields.
//
// Parameters:
//   - fieldName: The name of the spatial field
//   - shape: The shape to index
//
// Returns an error if the writer is closed, no strategy is registered for the field,
// or if the shape cannot be indexed.
func (w *SpatialIndexWriter) WriteSpatialField(fieldName string, shape Shape) ([]document.IndexableField, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, fmt.Errorf("SpatialIndexWriter is closed")
	}

	strategy, ok := w.strategies[fieldName]
	if !ok {
		return nil, fmt.Errorf("no spatial strategy registered for field %q", fieldName)
	}

	if shape == nil {
		return nil, fmt.Errorf("shape cannot be nil")
	}

	fields, err := strategy.CreateIndexableFields(shape)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexable fields for field %q: %w", fieldName, err)
	}

	return fields, nil
}

// WriteShape writes a shape using the specified field name.
// This is a convenience method that combines StartDocument, WriteSpatialField, and FinishDocument
// for simple single-shape documents.
//
// Parameters:
//   - fieldName: The name of the spatial field
//   - shape: The shape to index
//
// Returns the indexable fields created for the shape, or an error if the operation fails.
func (w *SpatialIndexWriter) WriteShape(fieldName string, shape Shape) ([]document.IndexableField, error) {
	if err := w.StartDocument(); err != nil {
		return nil, err
	}

	fields, err := w.WriteSpatialField(fieldName, shape)
	if err != nil {
		return nil, err
	}

	if err := w.FinishDocument(); err != nil {
		return nil, err
	}

	return fields, nil
}

// GetDirectory returns the directory used by this writer.
func (w *SpatialIndexWriter) GetDirectory() store.Directory {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.directory
}

// GetSegmentInfo returns the segment info for this writer.
func (w *SpatialIndexWriter) GetSegmentInfo() *index.SegmentInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.segmentInfo
}

// GetFieldInfos returns the field infos for this writer.
func (w *SpatialIndexWriter) GetFieldInfos() *index.FieldInfos {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.fieldInfos
}

// GetCurrentDoc returns the current document number being written.
func (w *SpatialIndexWriter) GetCurrentDoc() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.currentDoc
}

// GetDocCount returns the total number of documents written.
func (w *SpatialIndexWriter) GetDocCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.docCount
}

// IsClosed returns true if this writer has been closed.
func (w *SpatialIndexWriter) IsClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.closed
}

// Close releases all resources used by this writer.
// After closing, no further operations can be performed.
//
// Returns an error if the writer is already closed.
func (w *SpatialIndexWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("SpatialIndexWriter is already closed")
	}

	w.closed = true
	w.strategies = nil
	return nil
}

// GetRegisteredFields returns a list of field names that have registered strategies.
func (w *SpatialIndexWriter) GetRegisteredFields() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	fields := make([]string, 0, len(w.strategies))
	for fieldName := range w.strategies {
		fields = append(fields, fieldName)
	}
	return fields
}

// ClearStrategies removes all registered spatial strategies.
// This can be used to reset the writer for a new segment or configuration.
//
// Returns an error if the writer is closed.
func (w *SpatialIndexWriter) ClearStrategies() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("SpatialIndexWriter is closed")
	}

	w.strategies = make(map[string]SpatialStrategy)
	return nil
}

// SpatialIndexWriterState holds the current state of the writer.
// This can be used for monitoring and debugging purposes.
type SpatialIndexWriterState struct {
	CurrentDoc       int
	DocCount         int
	RegisteredFields []string
	Closed           bool
}

// GetState returns the current state of the writer.
func (w *SpatialIndexWriter) GetState() SpatialIndexWriterState {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return SpatialIndexWriterState{
		CurrentDoc:       w.currentDoc,
		DocCount:         w.docCount,
		RegisteredFields: w.GetRegisteredFields(),
		Closed:           w.closed,
	}
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// PointsFormat is an alias of [spi.PointsFormat]. rmp #4769 lifted the
// PointsFormat / PointsWriter / PointsReader family onto the canonical SPI
// surface (mirroring the KnnVectorsFormat lift in rmp #4707) so spi.Codec
// can expose PointsFormat(); the codecs-package interface collapsed to this
// pure alias. This is the Go port of org.apache.lucene.codecs.PointsFormat.
//
// Points are used for efficient range queries and spatial indexing. They are
// stored in a BKD (B-Tree K-D Tree) structure for fast intersection and range
// queries.
type PointsFormat = spi.PointsFormat

// BasePointsFormat provides common functionality for PointsFormat implementations.
type BasePointsFormat struct {
	name string
}

// NewBasePointsFormat creates a new BasePointsFormat.
func NewBasePointsFormat(name string) *BasePointsFormat {
	return &BasePointsFormat{name: name}
}

// Name returns the format name.
func (f *BasePointsFormat) Name() string {
	return f.name
}

// FieldsWriter returns a fields writer (must be implemented by subclasses).
func (f *BasePointsFormat) FieldsWriter(state *SegmentWriteState) (PointsWriter, error) {
	return nil, fmt.Errorf("FieldsWriter not implemented")
}

// FieldsReader returns a fields reader (must be implemented by subclasses).
func (f *BasePointsFormat) FieldsReader(state *SegmentReadState) (PointsReader, error) {
	return nil, fmt.Errorf("FieldsReader not implemented")
}

// PointsWriter is an alias of [spi.PointsWriter]. WriteField's fieldInfo
// parameter is *index.FieldInfo, which is itself an alias of
// *schema.FieldInfo (the type spi.PointsWriter declares), so existing
// implementations that spell the parameter *index.FieldInfo keep matching.
// This is the Go port of org.apache.lucene.codecs.PointsWriter.
type PointsWriter = spi.PointsWriter

// PointsReader is an alias of [spi.PointsReader]. The narrow SPI surface
// carries only CheckIntegrity / Close; the per-field getValues accessor
// lives on the concrete codecs-side reader. This is the Go port of
// org.apache.lucene.codecs.PointsReader.
type PointsReader = spi.PointsReader

// PointValues provides access to point values for a field.
// This is the Go port of Lucene's org.apache.lucene.index.PointValues.
type PointValues interface {
	// Intersect visits all points that intersect with the given visitor.
	Intersect(visitor IntersectVisitor) error

	// EstimatePointCount estimates the number of points in the given range.
	EstimatePointCount(visitor IntersectVisitor) int64

	// GetMinPackedValue returns the minimum packed value.
	GetMinPackedValue() []byte

	// GetMaxPackedValue returns the maximum packed value.
	GetMaxPackedValue() []byte

	// GetNumDimensions returns the number of dimensions.
	GetNumDimensions() int

	// GetBytesPerDimension returns the number of bytes per dimension.
	GetBytesPerDimension() int

	// GetDocCount returns the number of documents with points.
	GetDocCount() int
}

// IntersectVisitor is called during intersection queries.
// This is the Go port of Lucene's org.apache.lucene.index.PointValues.IntersectVisitor.
type IntersectVisitor interface {
	// Visit is called for each point that intersects the query.
	Visit(docID int) error

	// VisitByPackedValue is called for each point that intersects the query.
	VisitByPackedValue(docID int, packedValue []byte) error

	// Compare compares the given range with the query.
	// Returns the relation between the range and the query.
	Compare(minPackedValue, maxPackedValue []byte) Relation

	// Grow is called to grow the visitor's internal data structures.
	Grow(count int)
}

// Relation represents the relation between a range and a query.
type Relation int

const (
	// RelationCellOutsideQuery means the cell is outside the query.
	RelationCellOutsideQuery Relation = iota

	// RelationCellInsideQuery means the cell is inside the query.
	RelationCellInsideQuery

	// RelationCellCrossesQuery means the cell crosses the query boundary.
	RelationCellCrossesQuery
)

// String returns the string representation of the Relation.
func (r Relation) String() string {
	switch r {
	case RelationCellOutsideQuery:
		return "CELL_OUTSIDE_QUERY"
	case RelationCellInsideQuery:
		return "CELL_INSIDE_QUERY"
	case RelationCellCrossesQuery:
		return "CELL_CROSSES_QUERY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", r)
	}
}

// PointsWriterHelper is a helper for writing points.
type PointsWriterHelper struct {
	out    store.IndexOutput
	closed bool
}

// NewPointsWriterHelper creates a new PointsWriterHelper.
func NewPointsWriterHelper(out store.IndexOutput) *PointsWriterHelper {
	return &PointsWriterHelper{out: out}
}

// WriteHeader writes the points file header.
func (w *PointsWriterHelper) WriteHeader() error {
	// Write magic number (PT = Points)
	if err := store.WriteUint32(w.out, 0x50540000); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(w.out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close closes the writer.
func (w *PointsWriterHelper) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

// PointsReaderHelper is a helper for reading points.
type PointsReaderHelper struct {
	in     store.IndexInput
	closed bool
}

// NewPointsReaderHelper creates a new PointsReaderHelper.
func NewPointsReaderHelper(in store.IndexInput) *PointsReaderHelper {
	return &PointsReaderHelper{in: in}
}

// ReadHeader reads and validates the points file header.
func (r *PointsReaderHelper) ReadHeader() error {
	// Read magic number
	magic, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x50540000 {
		return fmt.Errorf("invalid magic number: expected 0x50540000, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close closes the reader.
func (r *PointsReaderHelper) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.in.Close()
}

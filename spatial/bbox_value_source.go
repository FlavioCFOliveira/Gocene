// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// BBoxValueSource returns a shape (bounding rectangle) for each document
// derived from the four numeric doc-values fields stored by BBoxStrategy.
//
// Port of org.apache.lucene.spatial.bbox.BBoxValueSource.
type BBoxValueSource struct {
	strategy *BBoxStrategy
}

// NewBBoxValueSource creates the value source bound to strategy.
func NewBBoxValueSource(strategy *BBoxStrategy) *BBoxValueSource {
	return &BBoxValueSource{strategy: strategy}
}

// GetStrategy returns the BBoxStrategy this source is bound to.
func (s *BBoxValueSource) GetStrategy() *BBoxStrategy { return s.strategy }

// String returns a human-readable description of the source.
func (s *BBoxValueSource) String() string {
	if s.strategy == nil {
		return "bboxShape()"
	}
	return "bboxShape(" + s.strategy.GetFieldName() + ")"
}

// numericDocValuesReader is the minimal interface used by BBoxValueSource
// to read numeric doc values from a leaf reader.
type numericDocValuesReader interface {
	GetNumericDocValues(field string) (index.NumericDocValues, error)
}

// GetValues returns a ShapeValues for each document by reading the four
// doc-value fields (minX, maxX, minY, maxY) from the given leaf context.
//
// The returned ShapeValues carries a Rectangle built from the four stored
// coordinates. Returns nil when the strategy is nil or the context does
// not expose the required doc-value fields.
func (s *BBoxValueSource) GetValues(ctx *index.LeafReaderContext) (*ShapeValues, error) {
	if s.strategy == nil || ctx == nil {
		return nil, nil
	}

	reader := ctx.Reader()
	if reader == nil {
		return nil, nil
	}

	dvReader, ok := reader.(numericDocValuesReader)
	if !ok {
		return nil, nil
	}

	return s.readBBox(dvReader)
}

// readBBox reads the four bounding-box fields from the doc-values reader
// and constructs a Rectangle ShapeValues.
func (s *BBoxValueSource) readBBox(dvReader numericDocValuesReader) (*ShapeValues, error) {
	minXValues, err := dvReader.GetNumericDocValues(s.strategy.minXFieldName)
	if err != nil || minXValues == nil {
		return nil, nil
	}
	maxXValues, err := dvReader.GetNumericDocValues(s.strategy.maxXFieldName)
	if err != nil || maxXValues == nil {
		return nil, nil
	}
	minYValues, err := dvReader.GetNumericDocValues(s.strategy.minYFieldName)
	if err != nil || minYValues == nil {
		return nil, nil
	}
	maxYValues, err := dvReader.GetNumericDocValues(s.strategy.maxYFieldName)
	if err != nil || maxYValues == nil {
		return nil, nil
	}

	// Advance all four iterators to the first document.
	docID, err := minXValues.NextDoc()
	if err != nil || docID < 0 {
		return nil, nil
	}
	if _, err := maxXValues.Advance(docID); err != nil {
		return nil, nil
	}
	if _, err := minYValues.Advance(docID); err != nil {
		return nil, nil
	}
	if _, err := maxYValues.Advance(docID); err != nil {
		return nil, nil
	}

	minX, err := minXValues.LongValue()
	if err != nil {
		return nil, nil
	}
	maxX, err := maxXValues.LongValue()
	if err != nil {
		return nil, nil
	}
	minY, err := minYValues.LongValue()
	if err != nil {
		return nil, nil
	}
	maxY, err := maxYValues.LongValue()
	if err != nil {
		return nil, nil
	}

	rect := &Rectangle{
		MinX: float64(minX),
		MaxX: float64(maxX),
		MinY: float64(minY),
		MaxY: float64(maxY),
	}

	return &ShapeValues{shape: rect}, nil
}

// IsCacheable reports whether this value source can be cached. BBoxValueSource
// is cacheable per leaf reader because it depends only on doc values.
func (s *BBoxValueSource) IsCacheable(_ interface{}) bool { return false }

// Equals reports whether other is an identical BBoxValueSource.
func (s *BBoxValueSource) Equals(other *BBoxValueSource) bool {
	if other == nil {
		return false
	}
	return s.strategy == other.strategy
}

// HashCode returns a hash based on the strategy's field name.
func (s *BBoxValueSource) HashCode() int {
	if s.strategy == nil {
		return 0
	}
	h := 31
	for _, c := range s.strategy.GetFieldName() {
		h = h*31 + int(c)
	}
	return h
}

// Ensure Rectangle implements Shape for BBoxValueSource.
var _ Shape = (*Rectangle)(nil)

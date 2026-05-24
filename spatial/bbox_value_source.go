// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

// BBoxValueSource returns a shape (bounding rectangle) for each document
// derived from the four numeric doc-values fields stored by BBoxStrategy.
//
// Port of org.apache.lucene.spatial.bbox.BBoxValueSource.
//
// Deviation: ShapeValuesSource, LeafReaderContext, and NumericDocValues are
// not yet ported; GetValues is a stub returning nil. Full implementation
// deferred to backlog #2693.
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

// GetValues returns nil — deferred to #2693.
func (s *BBoxValueSource) GetValues(_ interface{}) (interface{}, error) { return nil, nil }

// IsCacheable returns false — deferred to #2693.
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

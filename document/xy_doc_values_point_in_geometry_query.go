// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// XYDocValuesPointInGeometryQuery is the doc-values backed query that
// tests whether the per-document XY point lies inside any of the supplied
// XYGeometry shapes.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.XYDocValuesPointInGeometryQuery.
//
// Sprint 21 ships the structural surface (geometry list + field name +
// Equals/String). The actual document iteration scorer depends on
// search.ConstantScoreWeight + a sorted-numeric doc-values iterator and
// is deferred — backlog #2697.
type XYDocValuesPointInGeometryQuery struct {
	field      string
	geometries []geo.XYGeometry
}

// NewXYDocValuesPointInGeometryQuery constructs the query. The geometries
// slice must be non-empty.
func NewXYDocValuesPointInGeometryQuery(field string, geometries ...geo.XYGeometry) (*XYDocValuesPointInGeometryQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if len(geometries) == 0 {
		return nil, fmt.Errorf("at least one geometry is required")
	}
	for i, g := range geometries {
		if g == nil {
			return nil, fmt.Errorf("geometry at index %d is nil", i)
		}
	}
	dup := make([]geo.XYGeometry, len(geometries))
	copy(dup, geometries)
	return &XYDocValuesPointInGeometryQuery{field: field, geometries: dup}, nil
}

// Field returns the field name targeted by the query.
func (q *XYDocValuesPointInGeometryQuery) Field() string { return q.field }

// Geometries returns the geometry list as a defensive copy.
func (q *XYDocValuesPointInGeometryQuery) Geometries() []geo.XYGeometry {
	out := make([]geo.XYGeometry, len(q.geometries))
	copy(out, q.geometries)
	return out
}

// String returns a human-readable summary.
func (q *XYDocValuesPointInGeometryQuery) String() string {
	return fmt.Sprintf("XYDocValuesPointInGeometryQuery(field=%s, geometries=%d)", q.field, len(q.geometries))
}

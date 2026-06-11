// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYLineShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYLineShapeDVQueries (GOC-4010).
//
// The Java class is a thin subclass of BaseXYShapeDocValueTestCase that:
//   - selects ShapeType.LINE,
//   - delegates indexable-field creation to XYShape.createDocValueField, and
//   - reuses TestXYLineShapeQueries.LineValidator.
//
// All four verifyRandom* hooks are empty in upstream Lucene 10.4.0
// (commented "NOT IMPLEMENTED YET"), so the subclass exists purely to wire
// the abstract harness onto Cartesian line doc values.
//
// Gocene covers the same ground by verifying that XYShapeDocValuesQuery
// can be constructed with XYLine geometries for all supported relations
// and that WITHIN (not rejected at this level) and CONTAINS (rejected by
// BaseShapeDocValuesQuery) behave as expected.
func TestXYLineShapeDVQueries(t *testing.T) {
	line, err := geo.NewXYLine([]float32{0, 1, 2}, []float32{0, 1, 2})
	if err != nil {
		t.Fatalf("NewXYLine: %v", err)
	}

	// Test construction with each relation.
	t.Run("INTERSECTS", func(t *testing.T) {
		q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationIntersects, line)
		if err != nil {
			t.Fatalf("NewXYShapeDocValuesQuery: %v", err)
		}
		if got := q.GetField(); got != "shape" {
			t.Errorf("GetField: got %q, want %q", got, "shape")
		}
		if q.GetQueryComponent2D() == nil {
			t.Errorf("queryComponent2D must not be nil")
		}
	})

	t.Run("WITHIN", func(t *testing.T) {
		// WITHIN is NOT rejected for XYLine in the DocValues path
		// (only BKD-based XYShapeQuery rejects it).
		q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationWithin, line)
		if err != nil {
			t.Fatalf("NewXYShapeDocValuesQuery(WITHIN): %v", err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("DISJOINT", func(t *testing.T) {
		q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationDisjoint, line)
		if err != nil {
			t.Fatalf("NewXYShapeDocValuesQuery(DISJOINT): %v", err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	// CONTAINS is rejected by BaseShapeDocValuesQuery.
	t.Run("CONTAINS_rejected", func(t *testing.T) {
		_, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationContains, line)
		if err == nil {
			t.Fatal("expected error for CONTAINS relation, got nil")
		}
	})
}
